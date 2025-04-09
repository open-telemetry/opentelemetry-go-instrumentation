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
	Load(*link.Executable, *process.Info, *sampling.Config) error

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
	Uprobes []*Uprobe

	// SpecFn is a creation function for an eBPF CollectionSpec related to the
	// probe.
	SpecFn func() (*ebpf.CollectionSpec, error)
	// ProcessRecord is an optional processing function for the probe. If nil,
	// all records will be read directly into a new BPFEvent using the
	// encoding/binary package.
	ProcessRecord func(perf.Record) (*BPFEvent, error)

	reader          *perf.Reader
	collection      *ebpf.Collection
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
	var structFieldIDs []structfield.ID
	for _, cnst := range i.Consts {
		if sfc, ok := cnst.(StructFieldConst); ok {
			structFieldIDs = append(structFieldIDs, sfc.ID)
		}
		if sfc, ok := cnst.(StructFieldConstMinVersion); ok {
			structFieldIDs = append(structFieldIDs, sfc.StructField.ID)
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
func (i *Base[BPFObj, BPFEvent]) Load(
	exec *link.Executable,
	info *process.Info,
	sampler *sampling.Config,
) error {
	spec, err := i.SpecFn()
	if err != nil {
		return err
	}

	err = i.InjectConsts(info, spec)
	if err != nil {
		return err
	}

	i.collection, err = i.buildEBPFCollection(info, spec)
	if err != nil {
		return err
	}

	err = i.loadUprobes(exec, info)
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

func (i *Base[BPFObj, BPFEvent]) InjectConsts(info *process.Info, spec *ebpf.CollectionSpec) error {
	var err error
	var opts []inject.Option
	for _, cnst := range i.Consts {
		if l, ok := cnst.(setLogger); ok {
			cnst = l.SetLogger(i.Logger)
		}

		o, e := cnst.InjectOption(info)
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

func (i *Base[BPFObj, BPFEvent]) loadUprobes(exec *link.Executable, info *process.Info) error {
	for _, up := range i.Uprobes {
		var skip bool
		for _, pc := range up.PackageConstraints {
			if pc.Constraints.Check(info.Modules[pc.Package]) {
				continue
			}

			var logFn func(string, ...any)
			switch pc.FailureMode {
			case FailureModeIgnore:
				logFn = i.Logger.Debug
			case FailureModeWarn:
				logFn = i.Logger.Warn
			default:
				// Unknown and FailureModeError.
				return fmt.Errorf(
					"uprobe %s package constraint (%s) not met, version %v",
					up.Sym,
					pc.Constraints.String(),
					info.Modules[pc.Package],
				)
			}

			logFn(
				"package constraint not meet, skipping uprobe",
				"probe", i.ID,
				"symbol", up.Sym,
				"package", pc.Package,
				"constraint", pc.Constraints.String(),
				"version", info.Modules[pc.Package],
			)

			skip = true
			break
		}
		if skip {
			continue
		}

		err := up.load(exec, info, i.collection)
		if err != nil {
			var logFn func(string, ...any)
			switch up.FailureMode {
			case FailureModeIgnore:
				logFn = i.Logger.Debug
			case FailureModeWarn:
				logFn = i.Logger.Warn
			default:
				// Unknown and FailureModeError.
				return err
			}
			logFn("failed to load uprobe", "probe", i.ID, "symbol", up.Sym, "error", err)
			continue
		}
		i.closers = append(i.closers, up)
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

func (i *Base[BPFObj, BPFEvent]) buildEBPFCollection(
	info *process.Info,
	spec *ebpf.CollectionSpec,
) (*ebpf.Collection, error) {
	obj := new(BPFObj)
	if c, ok := ((interface{})(obj)).(io.Closer); ok {
		i.closers = append(i.closers, c)
	}

	sOpts := &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(info),
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
		if event == nil {
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
		if event == nil {
			continue
		}

		handle(i.ProcessFn(event))
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

	closers atomic.Pointer[[]io.Closer]
}

func (u *Uprobe) load(exec *link.Executable, info *process.Info, c *ebpf.Collection) error {
	offset, err := info.GetFunctionOffset(u.Sym)
	if err != nil {
		return err
	}

	var closers []io.Closer

	if u.EntryProbe != "" {
		entryProg, ok := c.Programs[u.EntryProbe]
		if !ok {
			return fmt.Errorf("entry probe %s not found", u.EntryProbe)
		}
		opts := &link.UprobeOptions{Address: offset, PID: int(info.ID)}
		l, err := exec.Uprobe("", entryProg, opts)
		if err != nil {
			return err
		}
		closers = append(closers, l)
	}

	if u.ReturnProbe != "" {
		retProg, ok := c.Programs[u.ReturnProbe]
		if !ok {
			return fmt.Errorf("return probe %s not found", u.ReturnProbe)
		}
		retOffsets, err := info.GetFunctionReturns(u.Sym)
		if err != nil {
			return err
		}

		for _, ret := range retOffsets {
			opts := &link.UprobeOptions{Address: ret, PID: int(info.ID)}
			l, err := exec.Uprobe("", retProg, opts)
			if err != nil {
				return err
			}
			closers = append(closers, l)
		}
	}

	old := u.closers.Swap(&closers)
	if old != nil {
		// load called twice without calling Close. Try and handle gracefully.
		var err error
		for _, closer := range *old {
			err = errors.Join(err, closer.Close())
		}
		return err
	}

	return nil
}

func (u *Uprobe) Close() error {
	closersPtr := u.closers.Swap(nil)
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

type setLogger interface {
	SetLogger(*slog.Logger) Const
}

// StructFieldConst is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package.
type StructFieldConst struct {
	Key string
	ID  structfield.ID

	logger *slog.Logger
}

var _ setLogger = StructFieldConst{}

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
