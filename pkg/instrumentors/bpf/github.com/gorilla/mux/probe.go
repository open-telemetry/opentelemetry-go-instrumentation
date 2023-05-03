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

package mux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"

	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in the gorilla/mux server during an HTTP
// request-response.
type Event struct {
	StartTime   uint64
	EndTime     uint64
	Method      [100]byte
	Path        [100]byte
	SpanContext context.EBPFSpanContext
}

// Instrumentor is the gorilla/mux instrumentor.
type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobe       link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the gorilla/mux package import path.
func (g *Instrumentor) LibraryName() string {
	return "github.com/gorilla/mux"
}

// FuncNames returns the function names from "github.com/gorilla/mux" that are
// instrumented.
func (g *Instrumentor) FuncNames() []string {
	return []string{"github.com/gorilla/mux.(*Router).ServeHTTP"}
}

// Load loads all instrumentation offsets.
func (g *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.StructField{
		{
			VarName:    "method_ptr_pos",
			StructName: "net/http.Request",
			Field:      "Method",
		},
		{
			VarName:    "url_ptr_pos",
			StructName: "net/http.Request",
			Field:      "URL",
		},
		{
			VarName:    "ctx_ptr_pos",
			StructName: "net/http.Request",
			Field:      "ctx",
		},
		{
			VarName:    "path_ptr_pos",
			StructName: "net/url.URL",
			Field:      "Path",
		},
	}, false)

	if err != nil {
		return err
	}

	g.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.BPFFsPath,
		},
	})
	if err != nil {
		return err
	}

	offset, err := ctx.TargetDetails.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}

	up, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		return err
	}

	g.uprobe = up
	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(g.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP_Returns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		g.returnProbs = append(g.returnProbs, retProbe)
	}

	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (g *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("gorilla/mux-instrumentor")
	var event Event
	for {
		record, err := g.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			logger.Error(err, "error parsing perf event")
			continue
		}

		eventsChan <- g.convertEvent(&event)
	}
}

func (g *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Library:     g.LibraryName(),
		Name:        path,
		Kind:        trace.SpanKindServer,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
		},
	}
}

// Close stops the Instrumentor.
func (g *Instrumentor) Close() {
	log.Logger.V(0).Info("closing gorilla/mux instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	if g.uprobe != nil {
		g.uprobe.Close()
	}

	for _, r := range g.returnProbs {
		r.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
