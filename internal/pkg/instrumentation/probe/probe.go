// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
	"go.opentelemetry.io/auto/pipeline"
)

// Probe is the instrument used by instrumentation for a Go package to measure
// and report on the state of that packages operation.
type Probe interface {
	// Manifest returns the Probe's instrumentation Manifest. This includes all
	// the information about the package the Probe instruments.
	Manifest() Manifest

	// InitStartupConfig sets up initialization config options for the Probe,
	// such as its sampling config, sets up its BPFObj as a closer, and initializes
	// the Probe's reader, returning it as an io.Closer.
	InitStartupConfig(*ebpf.Collection, *sampling.Config) (io.Closer, error)

	// Run runs the events processing loop.
	Run(*pipeline.Handler)

	// Spec returns the *ebpf.CollectionSpec for the Probe.
	Spec() (*ebpf.CollectionSpec, error)
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
	Uprobes []*Uprobe

	// SpecFn is a creation function for an eBPF CollectionSpec related to the
	// probe.
	SpecFn func() (*ebpf.CollectionSpec, error)
	// ProcessRecord is an optional processing function for the probe. If nil,
	// all records will be read directly into a new BPFEvent using the
	// encoding/binary package.
	ProcessRecord func(perf.Record) (*BPFEvent, error)

	reader          *perf.Reader
	closers         []io.Closer
	samplingManager *sampling.Manager
}

const (
	// PerfBufferDefaultSizeInPages is the default size of the perf buffer in
	// pages. We will need to make this configurable in the future.
	PerfBufferDefaultSizeInPages = 128
	// DefaultBufferMapName is the default name of the eBPF map used to pass
	// events from the eBPF program to userspace.
	DefaultBufferMapName = "events"
)

// Manifest returns the Probe's instrumentation Manifest.
func (i *Base[BPFObj, BPFEvent]) Manifest() Manifest {
	return NewManifest(i.ID, i.Consts, i.Uprobes)
}

func (i *Base[BPFObj, BPFEvent]) Spec() (*ebpf.CollectionSpec, error) {
	return i.SpecFn()
}

func (i *Base[BPFObj, BPFEvent]) InitStartupConfig(
	c *ebpf.Collection,
	sampler *sampling.Config,
) (io.Closer, error) {
	obj := new(BPFObj)
	if c, ok := ((interface{})(obj)).(io.Closer); ok {
		i.closers = append(i.closers, c)
	}

	samplingManager, err := sampling.NewSamplingManager(c, sampler)
	if err != nil {
		return nil, err
	}
	i.samplingManager = samplingManager

	buf, ok := c.Maps[DefaultBufferMapName]
	if !ok {
		return nil, fmt.Errorf("%s map not found", DefaultBufferMapName)
	}
	i.reader, err = perf.NewReader(buf, PerfBufferDefaultSizeInPages*os.Getpagesize())
	if err != nil {
		return nil, err
	}
	return i.reader, nil
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

	var event *BPFEvent
	if i.ProcessRecord != nil {
		event, err = i.ProcessRecord(record)
	} else {
		event = new(BPFEvent)
		buf := bytes.NewReader(record.RawSample)
		err = binary.Read(buf, binary.LittleEndian, event)
	}

	if err != nil {
		return nil, err
	}
	return event, nil
}

var _ Probe = &SpanProducer[struct{}, struct{}]{}

type SpanProducer[BPFObj any, BPFEvent any] struct {
	Base[BPFObj, BPFEvent]

	Version   string
	SchemaURL string
	ProcessFn func(*BPFEvent) ptrace.SpanSlice
}

// Run runs the events processing loop.
func (i *SpanProducer[BPFObj, BPFEvent]) Run(h *pipeline.Handler) {
	if h.TraceHandler == nil {
		i.Logger.Info("tracing not supported by handler, dropping traces", "handler", h)
		return
	}

	// Bind the single scope to the handler.
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("go.opentelemetry.io/auto/" + i.ID.InstrumentedPkg)
	scope.SetVersion(i.Version)
	handler := h.WithScope(scope, i.SchemaURL)

	for {
		event, err := i.read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}
		if event == nil {
			continue
		}

		handler.Trace(i.ProcessFn(event))
	}
}

type TraceProducer[BPFObj any, BPFEvent any] struct {
	Base[BPFObj, BPFEvent]

	ProcessFn func(*BPFEvent) (scope pcommon.InstrumentationScope, url string, spans ptrace.SpanSlice)
}

// Run runs the events processing loop.
func (i *TraceProducer[BPFObj, BPFEvent]) Run(h *pipeline.Handler) {
	th := h.TraceHandler
	if th == nil {
		i.Logger.Info("tracing not supported by handler, dropping traces", "handler", h)
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
		if event == nil {
			continue
		}

		scope, url, spans := i.ProcessFn(event)
		th.HandleTrace(scope, url, spans)
	}
}

// Uprobe is an eBPF program that is attached in the entry point and/or the return of a function.
type Uprobe struct {
	// Sym is the symbol name of the function to attach the eBPF program to.
	Sym string
	// PackageConstraints are the evaluated when the Uprobe is loaded.
	PackageConstraints []PackageConstraints
	// FailureMode defines the behavior that is performed when the Uprobe fails
	// to attach.
	FailureMode FailureMode
	// EntryProbe is the name of the eBPF program to attach to the entry of the
	// function specified by Sym. If EntryProbe is empty, no eBPF program will be attached to the entry of the function.
	EntryProbe string
	// ReturnProbe is the name of the eBPF program to attach to the return of the
	// function specified by Sym. If ReturnProbe is empty, no eBPF program will be attached to the return of the function.
	ReturnProbe string
	DependsOn   []string

	Closers atomic.Pointer[[]io.Closer]
}

func (u *Uprobe) Close() error {
	closersPtr := u.Closers.Swap(nil)
	if closersPtr == nil {
		// No closers.
		return nil
	}

	var err error
	for _, closer := range *closersPtr {
		err = errors.Join(err, closer.Close())
	}
	return err
}

// Const is an constant that needs to be injected into an eBPF program.
type Const interface {
	// InjectOption returns the inject.Option to run for the Const when running
	// inject.Constants.
	InjectOption(*process.Info) (inject.Option, error)
}

type SetLogger interface {
	SetLogger(*slog.Logger) Const
}

// StructFieldConst is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package.
type StructFieldConst struct {
	Key string
	ID  structfield.ID

	logger *slog.Logger
}

var _ SetLogger = StructFieldConst{}

// SetLogger sets the Logger for StructFieldConst operations.
func (c StructFieldConst) SetLogger(l *slog.Logger) Const {
	c.logger = l
	return c
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known. If it is not, an error is
// returned.
func (c StructFieldConst) InjectOption(info *process.Info) (inject.Option, error) {
	ver, ok := info.Modules[c.ID.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", c.ID.ModPath)
	}

	off, ok := inject.GetOffset(c.ID, ver)
	if !ok || !off.Valid {
		if c.logger != nil {
			c.logger.Info(
				"Offset not cached, analyzing directly",
				"key", c.Key,
				"id", c.ID,
			)
		}

		var err error
		off, err = inject.FindOffset(c.ID, info)
		if err != nil {
			return nil, fmt.Errorf("failed to find offset for %q: %w", c.ID, err)
		}
		if !off.Valid {
			return nil, fmt.Errorf("failed to find valid offset for %q", c.ID)
		}
	}

	if c.logger != nil {
		c.logger.Debug("Offset found", "key", c.Key, "id", c.ID, "offset", off.Offset)
	}
	return inject.WithKeyValue(c.Key, off.Offset), nil
}

// StructFieldConstMaxVersion is a [Const] for a struct field offset. These
// struct field ID needs to be known offsets in the [inject] package. The
// offset is only injected if the module version is less than the MaxVersion.
type StructFieldConstMaxVersion struct {
	StructField StructFieldConst
	// MaxVersion is the exclusive maximum version (it will only match versions
	// less than this).
	MaxVersion *semver.Version
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known and is less than the MaxVersion.
// If the module version is not known, an error is returned. If the module
// version is known but is greater than or equal to the MaxVersion, no offset
// is injected.
func (c StructFieldConstMaxVersion) InjectOption(info *process.Info) (inject.Option, error) {
	sf := c.StructField
	ver, ok := info.Modules[sf.ID.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", sf.ID.ModPath)
	}

	if !ver.LessThan(c.MaxVersion) {
		return nil, nil
	}

	return sf.InjectOption(info)
}

// StructFieldConstMinVersion is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package. The offset is only
// injected if the module version is greater than or equal to the MinVersion.
type StructFieldConstMinVersion struct {
	StructField StructFieldConst
	MinVersion  *semver.Version
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known and is greater than or equal to
// the MinVersion. If the module version is not known, an error is returned.
// If the module version is known but is less than the MinVersion, no offset is
// injected.
func (c StructFieldConstMinVersion) InjectOption(info *process.Info) (inject.Option, error) {
	sf := c.StructField
	ver, ok := info.Modules[sf.ID.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", sf.ID.ModPath)
	}

	if !ver.GreaterThanEqual(c.MinVersion) {
		return nil, nil
	}

	return sf.InjectOption(info)
}

// AllocationConst is a [Const] for all the allocation details that need to be
// injected into an eBPF program.
type AllocationConst struct {
	l *slog.Logger
}

// SetLogger sets the Logger for AllocationConst operations.
func (c AllocationConst) SetLogger(l *slog.Logger) Const {
	c.l = l
	return c
}

func (c AllocationConst) logger() *slog.Logger {
	l := c.l
	if l == nil {
		return slog.New(discardHandlerIntance)
	}
	return l
}

var discardHandlerIntance = discardHandler{}

// Copy of slog.DiscardHandler. Remove when support for Go < 1.24 is dropped.
type discardHandler struct{}

func (dh discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (dh discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (dh discardHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return dh }
func (dh discardHandler) WithGroup(name string) slog.Handler        { return dh }

// InjectOption returns the appropriately configured
// [inject.WithAllocation] if the [process.Allocation] within td
// are not nil. An error is returned if [process.Allocation] is nil.
func (c AllocationConst) InjectOption(info *process.Info) (inject.Option, error) {
	alloc, err := info.Alloc(c.logger())
	if err != nil {
		return nil, err
	}
	if alloc == nil {
		return nil, errors.New("no allocation details")
	}
	return inject.WithAllocation(*alloc), nil
}

// KeyValConst is a [Const] for a generic key-value pair.
//
// This should not be used as a replacement for any of the other provided
// [Const] implementations. Those implementations may have added significance
// and should be used instead where applicable.
type KeyValConst struct {
	Key string
	Val interface{}
}

// InjectOption returns the appropriately configured [inject.WithKeyValue].
func (c KeyValConst) InjectOption(*process.Info) (inject.Option, error) {
	return inject.WithKeyValue(c.Key, c.Val), nil
}
