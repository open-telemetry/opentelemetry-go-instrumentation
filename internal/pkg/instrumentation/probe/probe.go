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

// Probe is the instrument used by instrumentation for a Go package to measure
// and report on the state of that packages operation.
type Probe interface {
	// Manifest returns the Probe's instrumentation Manifest. This includes all
	// the information about the package the Probe instruments.
	Manifest() Manifest

	// Load loads all the eBPF programs and maps required by the Probe.
	// It also attaches the eBPF programs to the target process.
	// TODO: currently passing Sampler as an initial configuration - this will be
	// updated to a more generic configuration in the future.
	Load(*link.Executable, *process.TargetDetails, *sampling.Config) error

	// Run runs the events processing loop.
	Run(func(ptrace.ScopeSpans))

	// Close stops the Probe.
	Close() error
}

// Base is a base implementation of [Probe].
//
// This type can be returned by instrumentation directly. Instrumentation can
// also wrap this implementation with their own type if they need to override
// default behavior.
type Base[BPFObj any, BPFEvent any] struct {
	// ID is a unique identifier for the probe.
	ID ID
	// Logger is used to log operations and errors.
	Logger *slog.Logger

	// Consts are the constants that need to be injected into the eBPF program
	// that is run by this Probe.
	Consts []Const
	// Uprobes is a the collection of eBPF programs that need to be attached to
	// the target process.
	Uprobes []Uprobe

	// SpecFn is a creation function for an eBPF CollectionSpec related to the
	// probe.
	SpecFn func() (*ebpf.CollectionSpec, error)
	// ProcessRecord is an optional processing function for the probe. If nil,
	// all records will be read directly into a new BPFEvent using the
	// encoding/binary package.
	ProcessRecord func(perf.Record) (BPFEvent, error)

	reader          *perf.Reader
	collection      *ebpf.Collection
	closers         []io.Closer
	samplingManager *sampling.Manager
}

const (
	// The default size of the perf buffer in pages.
	// We will need to make this configurable in the future.
	PerfBufferDefaultSizeInPages = 128
	// The default name of the eBPF map used to pass events from the eBPF program
	// to userspace.
	DefaultBufferMapName = "events"
)

// Manifest returns the Probe's instrumentation Manifest.
func (i *Base[BPFObj, BPFEvent]) Manifest() Manifest {
	var structFieldIDs []structfield.ID
	for _, cnst := range i.Consts {
		if sfc, ok := cnst.(StructFieldConst); ok {
			structFieldIDs = append(structFieldIDs, sfc.Val)
		}
	}

	symbols := make([]FunctionSymbol, 0, len(i.Uprobes))
	for _, up := range i.Uprobes {
		symbols = append(symbols, FunctionSymbol{Symbol: up.Sym, DependsOn: up.DependsOn})
	}

	return NewManifest(i.ID, structFieldIDs, symbols)
}

func (i *Base[BPFObj, BPFEvent]) Spec() (*ebpf.CollectionSpec, error) {
	return i.SpecFn()
}

// Load loads all instrumentation offsets.
func (i *Base[BPFObj, BPFEvent]) Load(exec *link.Executable, td *process.TargetDetails, sampler *sampling.Config) error {
	spec, err := i.SpecFn()
	if err != nil {
		return err
	}

	err = i.InjectConsts(td, spec)
	if err != nil {
		return err
	}

	i.collection, err = i.buildEBPFCollection(td, spec)
	if err != nil {
		return err
	}

	err = i.loadUprobes(exec, td)
	if err != nil {
		return err
	}

	err = i.initReader()
	if err != nil {
		return err
	}

	i.samplingManager, err = sampling.NewSamplingManager(i.collection, sampler)
	if err != nil {
		return err
	}

	i.closers = append(i.closers, i.reader)

	return nil
}

func (i *Base[BPFObj, BPFEvent]) InjectConsts(td *process.TargetDetails, spec *ebpf.CollectionSpec) error {
	var err error
	var opts []inject.Option
	for _, cnst := range i.Consts {
		o, e := cnst.InjectOption(td)
		err = errors.Join(err, e)
		if e == nil && o != nil {
			opts = append(opts, o)
		}
	}
	if err != nil {
		return err
	}

	return inject.Constants(spec, opts...)
}

func (i *Base[BPFObj, BPFEvent]) loadUprobes(exec *link.Executable, td *process.TargetDetails) error {
	for _, up := range i.Uprobes {
		links, err := up.load(exec, td, i.collection)
		if err != nil {
			if up.Optional {
				i.Logger.Debug("failed to attach optional uprobe", "probe", i.ID, "symbol", up.Sym, "error", err)
				continue
			}
			return err
		}
		for _, l := range links {
			i.closers = append(i.closers, l)
		}
	}
	return nil
}

func (i *Base[BPFObj, BPFEvent]) initReader() error {
	buf, ok := i.collection.Maps[DefaultBufferMapName]
	if !ok {
		return fmt.Errorf("%s map not found", DefaultBufferMapName)
	}
	var err error
	i.reader, err = perf.NewReader(buf, PerfBufferDefaultSizeInPages*os.Getpagesize())
	if err != nil {
		return err
	}
	i.closers = append(i.closers, i.reader)
	return nil
}

func (i *Base[BPFObj, BPFEvent]) buildEBPFCollection(td *process.TargetDetails, spec *ebpf.CollectionSpec) (*ebpf.Collection, error) {
	obj := new(BPFObj)
	if c, ok := ((interface{})(obj)).(io.Closer); ok {
		i.closers = append(i.closers, c)
	}

	sOpts := &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(td),
		},
	}
	c, err := utils.InitializeEBPFCollection(spec, sOpts)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// read reads a new BPFEvent from the perf Reader.
func (i *Base[BPFObj, BPFEvent]) read() (*BPFEvent, error) {
	record, err := i.reader.Read()
	if err != nil {
		if !errors.Is(err, perf.ErrClosed) {
			i.Logger.Error("error reading from perf reader", "error", err)
		}
		return nil, err
	}

	if record.LostSamples != 0 {
		i.Logger.Debug("perf event ring buffer full", "dropped", record.LostSamples)
		return nil, err
	}

	var event BPFEvent
	if i.ProcessRecord != nil {
		event, err = i.ProcessRecord(record)
	} else {
		buf := bytes.NewReader(record.RawSample)
		err = binary.Read(buf, binary.LittleEndian, &event)
	}

	if err != nil {
		return nil, err
	}
	return &event, nil
}

// Close stops the Probe.
func (i *Base[BPFObj, BPFEvent]) Close() error {
	if i.collection != nil {
		i.collection.Close()
	}
	var err error
	for _, c := range i.closers {
		err = errors.Join(err, c.Close())
	}
	if err == nil {
		i.Logger.Debug("Closed", "Probe", i.ID)
	}
	return err
}

type SpanProducer[BPFObj any, BPFEvent any] struct {
	Base[BPFObj, BPFEvent]

	Version   string
	SchemaURL string
	ProcessFn func(*BPFEvent) ptrace.SpanSlice
}

// Run runs the events processing loop.
func (i *SpanProducer[BPFObj, BPFEvent]) Run(handle func(ptrace.ScopeSpans)) {
	for {
		event, err := i.read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}

		ss := ptrace.NewScopeSpans()

		ss.Scope().SetName("go.opentelemetry.io/auto/" + i.ID.InstrumentedPkg)
		ss.Scope().SetVersion(i.Version)
		ss.SetSchemaUrl(i.SchemaURL)

		i.ProcessFn(event).CopyTo(ss.Spans())

		handle(ss)
	}
}

type TraceProducer[BPFObj any, BPFEvent any] struct {
	Base[BPFObj, BPFEvent]

	ProcessFn func(*BPFEvent) ptrace.ScopeSpans
}

// Run runs the events processing loop.
func (i *TraceProducer[BPFObj, BPFEvent]) Run(handle func(ptrace.ScopeSpans)) {
	for {
		event, err := i.read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}

		handle(i.ProcessFn(event))
	}
}

// Uprobe is an eBPF program that is attached in the entry point and/or the return of a function.
type Uprobe struct {
	// Sym is the symbol name of the function to attach the eBPF program to.
	Sym string
	// Optional is a boolean flag informing if the Uprobe is optional. If the
	// Uprobe is optional and fails to attach, the error is logged and
	// processing continues.
	Optional bool
	// EntryProbe is the name of the eBPF program to attach to the entry of the
	// function specified by Sym. If EntryProbe is empty, no eBPF program will be attached to the entry of the function.
	EntryProbe string
	// ReturnProbe is the name of the eBPF program to attach to the return of the
	// function specified by Sym. If ReturnProbe is empty, no eBPF program will be attached to the return of the function.
	ReturnProbe string
	DependsOn   []string
}

func (u *Uprobe) load(exec *link.Executable, target *process.TargetDetails, c *ebpf.Collection) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(u.Sym)
	if err != nil {
		return nil, err
	}

	var links []link.Link

	if u.EntryProbe != "" {
		entryProg, ok := c.Programs[u.EntryProbe]
		if !ok {
			return nil, fmt.Errorf("entry probe %s not found", u.EntryProbe)
		}
		opts := &link.UprobeOptions{Address: offset, PID: target.PID}
		l, err := exec.Uprobe("", entryProg, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	if u.ReturnProbe != "" {
		retProg, ok := c.Programs[u.ReturnProbe]
		if !ok {
			return nil, fmt.Errorf("return probe %s not found", u.ReturnProbe)
		}
		retOffsets, err := target.GetFunctionReturns(u.Sym)
		if err != nil {
			return nil, err
		}

		for _, ret := range retOffsets {
			opts := &link.UprobeOptions{Address: ret, PID: target.PID}
			l, err := exec.Uprobe("", retProg, opts)
			if err != nil {
				return nil, err
			}
			links = append(links, l)
		}
	}

	return links, nil
}
