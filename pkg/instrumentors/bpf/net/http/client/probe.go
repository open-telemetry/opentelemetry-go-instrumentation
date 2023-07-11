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

package client

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
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in an HTTP server during an HTTP
// request-response.
type Event struct {
	StartTime         uint64
	EndTime           uint64
	Method            [10]byte
	Path              [100]byte
	SpanContext       context.EBPFSpanContext
	ParentSpanContext context.EBPFSpanContext
}

// Instrumentor is the net/http instrumentor.
type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the net/http package name.
func (h *Instrumentor) LibraryName() string {
	return "net/http/client"
}

// FuncNames returns the function names from "net/http" that are instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"net/http.(*Client).do"}
}

// Load loads all instrumentation offsets.
func (h *Instrumentor) Load(ctx *context.InstrumentorContext) error {
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
			VarName:    "path_ptr_pos",
			StructName: "net/url.URL",
			Field:      "Path",
		},
		{
			VarName:    "headers_ptr_pos",
			StructName: "net/http.Request",
			Field:      "Header",
		},
		{
			VarName:    "ctx_ptr_pos",
			StructName: "net/http.Request",
			Field:      "ctx",
		},
	}, true)

	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}
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
		return err
	}

	up, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeHttpClientDo, &link.UprobeOptions{
		Address: offset,
	})

	if err != nil {
		return err
	}

	h.uprobes = append(h.uprobes, up)

	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(h.FuncNames()[0])

	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeHttpClientDoReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}

	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	h.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (h *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("net/http/client-instrumentor")
	var event Event
	for {
		record, err := h.eventsReader.Read()
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

		eventsChan <- h.convertEvent(&event)
	}
}

func (h *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	var pscPtr *trace.SpanContext
	if e.ParentSpanContext.TraceID.IsValid() {
		psc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    e.ParentSpanContext.TraceID,
			SpanID:     e.ParentSpanContext.SpanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		pscPtr = &psc
	} else {
		pscPtr = nil
	}

	return &events.Event{
		Library:     h.LibraryName(),
		Name:        path,
		Kind:        trace.SpanKindClient,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
		},
		ParentSpanContext: pscPtr,
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	log.Logger.V(0).Info("closing net/http/client instrumentor")
	if h.eventsReader != nil {
		h.eventsReader.Close()
	}

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
