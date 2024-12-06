// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// ErrInvalidConfig is intended to be used by a Probe's implementation of
// ApplyConfig() when the provided probe.Config interface does not convert to
// the Probe's custom config struct. For example, libraries may choose to ignore
// this error when batch-updating Probes with a value that doesn't apply to all Probes.
var ErrInvalidConfig = errors.New("invalid config type for Probe")

const (
	// The default size of the perf buffer in pages.
	// We will need to make this configurable in the future.
	PerfBufferDefaultSizeInPages = 128
	// The default name of the eBPF map used to pass events from the eBPF program
	// to userspace.
	DefaultBufferMapName = "events"
)

// TracingConfig provides base configuration options for trace-based Probes.
type TracingConfig struct {
	SamplingConfig *sampling.Config
}

// TargetExecutableConfig provides executable and target process details for applications.
type TargetExecutableConfig struct {
	// Executable defines a target executable.
	Executable *link.Executable
	// TargetDetails defines target process information.
	TargetDetails *process.TargetDetails
}

// BasicProbe is a provided implementation of BaseProbe. It is the base configuration
// and identification layer for any Probe.
type BasicProbe struct {
	// ProbeID is a unique identifier for the Probe.
	ProbeID ID
	// Logger is used to log operations and errors.
	Logger *slog.Logger
}

// ID returns the ID for this Probe.
func (b *BasicProbe) ID() ID {
	return b.ProbeID
}

// GetLogger returns the Logger for this Probe.
func (b *BasicProbe) GetLogger() *slog.Logger {
	return b.Logger
}

// TargetExecutableProbe is a provided implementation of GoLibraryTelemetryProbe.
type TargetExecutableProbe[BPFObj any] struct {
	*BasicProbe
	*TargetExecutableConfig

	// Consts are the constants that need to be injected into the eBPF program
	// that is run by this Probe. Currently only used by TargetingProbes.
	Consts []Const

	// SpecFn is a creation function for an eBPF CollectionSpec related to the
	// probe.
	SpecFn func() (*ebpf.CollectionSpec, error)

	closers    []io.Closer
	collection *ebpf.Collection
	reader     *perf.Reader
	// Uprobes is a the collection of eBPF programs that need to be attached to
	// the target process.
	Uprobes []Uprobe
}

func (p *TargetExecutableProbe[BPFObj]) TargetConfig() *TargetExecutableConfig {
	return p.TargetExecutableConfig
}

// Load loads the eBPF programs for this Probe into memory.
func (p *TargetExecutableProbe[BPFObj]) Load() error {
	spec, err := p.SpecFn()
	if err != nil {
		return err
	}

	// Inject Consts into the Probe's collection spec.
	var opts []inject.Option
	for _, cnst := range p.Consts {
		o, e := cnst.InjectOption(p.TargetDetails)
		err = errors.Join(err, e)
		if e == nil && o != nil {
			opts = append(opts, o)
		}
	}
	if err != nil {
		return err
	}
	err = inject.Constants(spec, opts...)
	if err != nil {
		return err
	}

	// Set up closers for the Probe
	obj := new(BPFObj)
	if c, ok := ((interface{})(obj)).(io.Closer); ok {
		p.closers = append(p.closers, c)
	}

	// Initialize the eBPF collection for the Probe
	sOpts := &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(p.TargetDetails),
		},
	}
	c, err := utils.InitializeEBPFCollection(spec, sOpts)
	if err != nil {
		return err
	}
	p.collection = c

	return nil
}

// Attach attaches loaded eBPF programs to trigger points and initializes the Probe.
func (p *TargetExecutableProbe[BPFObj]) Attach() error {
	// Attach Uprobes
	for _, up := range p.Uprobes {
		links, err := up.load(p.Executable, p.TargetDetails, p.collection)
		if err != nil {
			if up.Optional {
				p.Logger.Debug("failed to attach optional uprobe", "probe", p.ID, "symbol", up.Sym, "error", err)
				continue
			}
			return err
		}
		for _, l := range links {
			p.closers = append(p.closers, l)
		}
	}

	// Initialize reader for the Probe
	buf, ok := p.collection.Maps[DefaultBufferMapName]
	if !ok {
		return fmt.Errorf("%s map not found", DefaultBufferMapName)
	}
	var err error
	p.reader, err = perf.NewReader(buf, PerfBufferDefaultSizeInPages*os.Getpagesize())
	if err != nil {
		return err
	}
	p.closers = append(p.closers, p.reader)

	return nil
}

// Manifest returns the Probe's instrumentation Manifest.
func (p *TargetExecutableProbe[BPFObj]) Manifest() Manifest {
	var structFieldIDs []structfield.ID
	for _, cnst := range p.Consts {
		if sfc, ok := cnst.(StructFieldConst); ok {
			structFieldIDs = append(structFieldIDs, sfc.Val)
		}
	}

	symbols := make([]FunctionSymbol, 0, len(p.Uprobes))
	for _, up := range p.Uprobes {
		symbols = append(symbols, FunctionSymbol{Symbol: up.Sym, DependsOn: up.DependsOn})
	}

	return NewManifest(p.ProbeID, structFieldIDs, symbols)
}

func (p *TargetExecutableProbe[BPFObj]) Close() error {
	if p.collection != nil {
		p.collection.Close()
	}
	var err error
	for _, c := range p.closers {
		err = errors.Join(err, c.Close())
	}
	if err == nil {
		p.Logger.Debug("Closed", "Probe", p.ID)
	}
	return err
}

// TargetEventProducingProbe is a TargetExecutableProbe that reads and processes eBPF events.
type TargetEventProducingProbe[BPFObj any, BPFEvent any] struct {
	*TargetExecutableProbe[BPFObj]

	// ProcessRecord is an optional processing function for the probe. If nil,
	// all records will be read directly into a new BPFEvent using the
	// encoding/binary package.
	ProcessRecord func(perf.Record) (BPFEvent, error)
}

// read reads a new BPFEvent from the perf Reader.
func (e *TargetEventProducingProbe[BPFObj, BPFEvent]) read() (*BPFEvent, error) {
	record, err := e.reader.Read()
	if err != nil {
		if !errors.Is(err, perf.ErrClosed) {
			e.Logger.Error("error reading from perf reader", "error", err)
		}
		return nil, err
	}

	if record.LostSamples != 0 {
		e.Logger.Debug("perf event ring buffer full", "dropped", record.LostSamples)
		return nil, err
	}

	var event BPFEvent
	if e.ProcessRecord != nil {
		event, err = e.ProcessRecord(record)
	} else {
		buf := bytes.NewReader(record.RawSample)
		err = binary.Read(buf, binary.LittleEndian, &event)
	}

	if err != nil {
		return nil, err
	}
	return &event, nil
}

// TargetSpanProducingProbe is a provided implementation of GoLibraryTelemetryProbe that
// processes and handles ptrace.ScopeSpans by emitting spans.
type TargetSpanProducingProbe[BPFObj any, BPFEvent any] struct {
	*TargetEventProducingProbe[BPFObj, BPFEvent]
	*TracingConfig

	Version   string
	SchemaURL string
	ProcessFn func(*BPFEvent) ptrace.SpanSlice

	Handler func(ptrace.ScopeSpans)
}

func (s *TargetSpanProducingProbe[BPFObj, BPFEvent]) TraceConfig() *TracingConfig {
	return s.TracingConfig
}

func (s *TargetSpanProducingProbe[BPFObj, BPFEvent]) Run() {
	_, err := sampling.NewSamplingManager(s.collection, s.SamplingConfig)
	if err != nil {
		s.Logger.Error("unable to get new sampling manager", "err", err)
		return
	}

	for {
		event, err := s.read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}

		ss := ptrace.NewScopeSpans()

		ss.Scope().SetName("go.opentelemetry.io/auto/" + s.ProbeID.InstrumentedPkg)
		ss.Scope().SetVersion(s.Version)
		ss.SetSchemaUrl(s.SchemaURL)

		s.ProcessFn(event).CopyTo(ss.Spans())

		s.Handler(ss)
	}
}

// TargetTraceProducingProbe is a provided implementation of GoLibraryTelemetryProbe that
// processes and handles ptrace.ScopeSpans by emitting traces.
type TargetTraceProducingProbe[BPFObj any, BPFEvent any] struct {
	*TargetEventProducingProbe[BPFObj, BPFEvent]
	*TracingConfig

	ProcessFn func(*BPFEvent) ptrace.ScopeSpans

	Handler func(ptrace.ScopeSpans)
}

// Run runs the events processing loop.
func (i *TargetTraceProducingProbe[BPFObj, BPFEvent]) Run() {
	_, err := sampling.NewSamplingManager(i.collection, i.SamplingConfig)
	if err != nil {
		i.Logger.Error("unable to get new sampling manager", "err", err)
		return
	}

	for {
		event, err := i.read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}

		i.Handler(i.ProcessFn(event))
	}
}
