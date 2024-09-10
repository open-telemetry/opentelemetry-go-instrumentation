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
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"

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
	Run(eventsChan chan<- *Event)

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
	Logger logr.Logger

	// Consts are the constants that need to be injected into the eBPF program
	// that is run by this Probe.
	Consts []Const
	// Uprobes is a the collection of eBPF programs that need to be attached to
	// the target process.
	Uprobes []Uprobe

	// SpecFn is a creation function for an eBPF CollectionSpec related to the
	// probe.
	SpecFn func() (*ebpf.CollectionSpec, error)
	// ProcessFn processes probe events into a uniform Event type.
	ProcessFn func(*BPFEvent) []*SpanEvent

	reader     *perf.Reader
	collection *ebpf.Collection
	closers    []io.Closer
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
	structfields := consts(i.Consts).structFields()

	symbols := make([]FunctionSymbol, 0, len(i.Uprobes))
	for _, up := range i.Uprobes {
		symbols = append(symbols, FunctionSymbol{Symbol: up.Sym, DependsOn: up.DependsOn})
	}

	return NewManifest(i.ID, structfields, symbols)
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

	// TODO: Initialize sampling manager based on the sampling configuration and the eBPF collection.
	// The manager will be responsible for writing to eBPF maps - configuring the sampling.
	// In addition the sampling manager will be responsible for handling updates for the configuration.

	i.closers = append(i.closers, i.reader)

	return nil
}

func (i *Base[BPFObj, BPFEvent]) InjectConsts(td *process.TargetDetails, spec *ebpf.CollectionSpec) error {
	opts, err := consts(i.Consts).injectOpts(td)
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
				i.Logger.V(1).Info("failed to attach optional uprobe", "probe", i.ID, "symbol", up.Sym, "error", err)
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

// Run runs the events processing loop.
func (i *Base[BPFObj, BPFEvent]) Run(dest chan<- *Event) {
	for {
		record, err := i.reader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			i.Logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			i.Logger.V(1).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		se, err := i.processRecord(record)
		if err != nil {
			i.Logger.Error(err, "failed to process perf record")
		}
		e := &Event{
			Package:    i.ID.InstrumentedPkg,
			Kind:       i.ID.SpanKind,
			SpanEvents: se,
		}

		dest <- e
	}
}

func (i *Base[BPFObj, BPFEvent]) processRecord(record perf.Record) ([]*SpanEvent, error) {
	buf := bytes.NewBuffer(record.RawSample)

	var event BPFEvent
	err := binary.Read(buf, binary.LittleEndian, &event)
	if err != nil {
		return nil, err
	}
	return i.ProcessFn(&event), nil
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
		i.Logger.V(1).Info("Closed", "Probe", i.ID)
	}
	return err
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

// Const is an constant that needs to be injected into an eBPF program.
type Const interface {
	// InjectOption returns the inject.Option to run for the Const when running
	// inject.Constants.
	InjectOption(td *process.TargetDetails) (inject.Option, error)
}

type consts []Const

func (c consts) structFields() []structfield.ID {
	var out []structfield.ID
	for _, cnst := range c {
		if sfc, ok := cnst.(StructFieldConst); ok {
			out = append(out, sfc.Val)
		}
	}
	return out
}

func (c consts) injectOpts(td *process.TargetDetails) ([]inject.Option, error) {
	var (
		out []inject.Option
		err error
	)
	for _, cnst := range c {
		o, e := cnst.InjectOption(td)
		err = errors.Join(err, e)
		if e == nil && o != nil {
			out = append(out, o)
		}
	}
	return out, err
}

// StructFieldConst is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package.
type StructFieldConst struct {
	Key string
	Val structfield.ID
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known. If it is not, an error is
// returned.
func (c StructFieldConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	ver, ok := td.Libraries[c.Val.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", c.Val.ModPath)
	}
	return inject.WithOffset(c.Key, c.Val, ver), nil
}

// StructFieldConstMinVersion is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package. The offset is only
// injected if the module version is greater than or equal to the MinVersion.
type StructFieldConstMinVersion struct {
	StructField StructFieldConst
	MinVersion  *version.Version
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known and is greater than or equal to
// the MinVersion. If the module version is not known, an error is returned.
// If the module version is known but is less than the MinVersion, no offset is
// injected.
func (c StructFieldConstMinVersion) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	sf := c.StructField
	ver, ok := td.Libraries[sf.Val.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", sf.Val.ModPath)
	}

	if !ver.GreaterThanOrEqual(c.MinVersion) {
		return nil, nil
	}

	return inject.WithOffset(sf.Key, sf.Val, ver), nil
}

// AllocationConst is a [Const] for all the allocation details that need to be
// injected into an eBPF program.
type AllocationConst struct{}

// InjectOption returns the appropriately configured
// [inject.WithAllocationDetails] if the [process.AllocationDetails] within td
// are not nil. An error is returned if [process.AllocationDetails] is nil.
func (c AllocationConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	if td.AllocationDetails == nil {
		return nil, errors.New("no allocation details")
	}
	return inject.WithAllocationDetails(*td.AllocationDetails), nil
}

// RegistersABIConst is a [Const] for the boolean flag informing an eBPF
// program if the Go space has registered ABI.
type RegistersABIConst struct{}

// InjectOption returns the appropriately configured [inject.WithRegistersABI].
func (c RegistersABIConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	return inject.WithRegistersABI(td.IsRegistersABI()), nil
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
func (c KeyValConst) InjectOption(*process.TargetDetails) (inject.Option, error) {
	return inject.WithKeyValue(c.Key, c.Val), nil
}
