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

package sql

import (
	// "bytes"
	// "encoding/binary"
	// "errors"
	//"os"
	"fmt"
	"time" // todo: remove

	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	//"github.com/cilium/ebpf/perf"
	//"golang.org/x/sys/unix"

	//"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/log"
	//"go.opentelemetry.io/otel/attribute"
	//"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in an SQL database
// request-response.
type Event struct {
	StartTime         uint64
	EndTime           uint64
	Method            [10]byte
	Path              [100]byte
	SpanContext       context.EBPFSpanContext
	ParentSpanContext context.EBPFSpanContext
}

// Instrumentor is the database/sql instrumentor.
type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	//eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the database/sql/ package name.
func (h *Instrumentor) LibraryName() string {
	return "database/sql/sql"
}

// FuncNames returns the function names from "database/sql/" that are instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"database/sql.(*Conn).QueryContext"}
}

// Load loads all instrumentation offsets.
func (h *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}
	fmt.Printf("hi33")
	h.bpfObjects = &bpfObjects{}
	fmt.Printf("hi34")
	err = spec.LoadAndAssign(h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.BPFFsPath,
		},
	})

	if err != nil {
		return err
	}

	offset, err := ctx.TargetDetails.GetFunctionOffset(h.FuncNames()[0])

	if err != nil {
		fmt.Printf("hi1")
		return err
	}

	up, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeQueryContext, &link.UprobeOptions{
		Address: offset,
	})

	if err != nil {
		fmt.Printf("hi2")
		return err
	}

	h.uprobes = append(h.uprobes, up)

	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(h.FuncNames()[0])

	if err != nil {
		fmt.Printf("hi3")
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", h.bpfObjects.UuprobeQueryContextReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			fmt.Printf("hi4")
			return err
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}

	// rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	// if err != nil {
	// 	return err
	// }
	// h.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (h *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("database/sql/sql-instrumentor")
	logger.V(0).Info("Inside function")
	//var event Event
	for {
		time.Sleep(3 * time.Second)
		// record, err := h.eventsReader.Read()
		// if err != nil {
		// 	if errors.Is(err, perf.ErrClosed) {
		// 		return
		// 	}
		// 	logger.Error(err, "error reading from perf reader")
		// 	continue
		// }

		// if record.LostSamples != 0 {
		// 	logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
		// 	continue
		// }

		// if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
		// 	logger.Error(err, "error parsing perf event")
		// 	continue
		// }
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	log.Logger.V(0).Info("closing database/sql/sql instrumentor")
	// if h.eventsReader != nil {
	// 	h.eventsReader.Close()
	// }

	for _, r := range h.uprobes {
		r.Close()
	}

	for _, r := range h.returnProbs {
		r.Close()
	}

	if h.bpfObjects != nil {
		h.bpfObjects.Close()
	}
}
