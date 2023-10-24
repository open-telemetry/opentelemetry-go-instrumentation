// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Probe is the instrument used by instrumentation for a Go package to measure
// and report on the state of that packages operation.
type Probe interface {
	// Manifest returns the Instrumentor's instrumentation Manifest. This
	// includes all the information about the package the Instrumentor
	// instruments.
	Manifest() Manifest

	// Load loads all instrumentation offsets.
	Load(*link.Executable, *process.TargetDetails) error

	// Run runs the events processing loop.
	Run(eventsChan chan<- *Event)

	// Close stops the Probe.
	Close()
}

// UprobeFunc is a function that will attach a eBPF program to a perf event
// that fires when the given symbol starts executing in exec.
//
// It is expected the symbol belongs to are shared library and its offset can
// be determined using target.
//
// Losing the reference to the resulting Link (up) will close the Uprobe and
// prevent further execution of prog. The Link must be Closed during program
// shutdown to avoid leaking system resources.
type UprobeFunc[BPFObj any] func(symbol string, exec *link.Executable, target *process.TargetDetails, obj *BPFObj) ([]link.Link, error)

type Base[BPFObj any, BPFEvent any] struct {
	Name string
	// ModPath is the module path of the instrumented code.
	//
	// For instrumentation of standard library Go packages, use "std".
	ModPath string
	Logger  logr.Logger

	Consts []Const
	// Uprobes is a mapping from runtime symbols to a UprobeFunc.
	Uprobes map[string]UprobeFunc[BPFObj]

	ReaderFn  func(BPFObj) (*perf.Reader, error)
	SpecFn    func() (*ebpf.CollectionSpec, error)
	ProcessFn func(*BPFEvent) *Event

	reader  *perf.Reader
	closers []io.Closer
}

func (i *Base[BPFObj, BPFEvent]) Manifest() Manifest {
	structfields := consts(i.Consts).structFields()

	symbols := make([]string, 0, len(i.Uprobes))
	for s := range i.Uprobes {
		symbols = append(symbols, s)
	}

	return NewManifest(i.Name, i.ModPath, structfields, symbols)
}

func (i *Base[BPFObj, BPFEvent]) Load(exec *link.Executable, td *process.TargetDetails) error {
	spec, err := i.SpecFn()
	if err != nil {
		return err
	}

	err = i.injectConsts(td, spec)
	if err != nil {
		return err
	}

	obj, err := i.buildObj(exec, td, spec)
	if err != nil {
		return err
	}

	i.reader, err = i.ReaderFn(*obj)
	if err != nil {
		return err
	}
	i.closers = append(i.closers, i.reader)

	return nil
}

func (i *Base[BPFObj, BPFEvent]) injectConsts(td *process.TargetDetails, spec *ebpf.CollectionSpec) error {
	opts, err := consts(i.Consts).injectOpts(td)
	if err != nil {
		return err
	}
	return inject.Constants(spec, opts...)
}

func (i *Base[BPFObj, BPFEvent]) buildObj(exec *link.Executable, td *process.TargetDetails, spec *ebpf.CollectionSpec) (*BPFObj, error) {
	obj := new(BPFObj)
	if c, ok := ((interface{})(obj)).(io.Closer); ok {
		i.closers = append(i.closers, c)
	}

	sOpts := &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(td),
		},
	}
	err := utils.LoadEBPFObjects(spec, obj, sOpts)
	if err != nil {
		return nil, err
	}

	for symb, f := range i.Uprobes {
		links, err := f(symb, exec, td, obj)
		if err != nil {
			return nil, err
		}
		for _, l := range links {
			i.closers = append(i.closers, l)
		}
	}

	return obj, nil
}

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
			i.Logger.Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		e, err := i.processRecord(record)
		if err != nil {
			i.Logger.Error(err, "failed to process perf record")
		}

		dest <- e
	}
}

func (i *Base[BPFObj, BPFEvent]) processRecord(record perf.Record) (*Event, error) {
	buf := bytes.NewBuffer(record.RawSample)

	var event BPFEvent
	err := binary.Read(buf, binary.LittleEndian, &event)
	if err != nil {
		return nil, err
	}
	return i.ProcessFn(&event), nil
}

func (i *Base[BPFObj, BPFEvent]) Close() {
	var err error
	for _, c := range i.closers {
		err = errors.Join(err, c.Close())
	}
	if err != nil {
		i.Logger.Error(err, "failed to cleanup", "Probe", i.Name)
	}
}
