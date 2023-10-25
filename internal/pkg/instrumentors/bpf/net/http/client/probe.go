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

	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in an HTTP server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	Method [10]byte
	Path   [100]byte
}

// Instrumentor is the net/http instrumentor.
type Instrumentor struct {
	logger       logr.Logger
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New(logger logr.Logger) *Instrumentor {
	return &Instrumentor{logger: logger.WithName("Instrumentor/HTTP/Client")}
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
func (h *Instrumentor) Load(exec *link.Executable, target *process.TargetDetails) error {
	ver := target.GoVersion

	spec, err := loadBpf()
	if err != nil {
		return err
	}
	if target.AllocationDetails == nil {
		// This Instrumentor requires allocation.
		return errors.New("no allocation details")
	}
	err = inject.Constants(
		spec,
		inject.WithRegistersABI(target.IsRegistersABI()),
		inject.WithAllocationDetails(*target.AllocationDetails),
		inject.WithOffset("method_ptr_pos", "net/http.Request", "Method", ver),
		inject.WithOffset("url_ptr_pos", "net/http.Request", "URL", ver),
		inject.WithOffset("path_ptr_pos", "net/url.URL", "Path", ver),
		inject.WithOffset("headers_ptr_pos", "net/http.Request", "Header", ver),
		inject.WithOffset("ctx_ptr_pos", "net/http.Request", "ctx", ver),
		inject.WithOffset("buckets_ptr_pos", "runtime.hmap", "buckets", ver),
	)
	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}
	err = spec.LoadAndAssign(h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(target),
		},
	})

	if err != nil {
		return err
	}

	offset, err := target.GetFunctionOffset(h.FuncNames()[0])
	if err != nil {
		return err
	}

	up, err := exec.Uprobe("", h.bpfObjects.UprobeHttpClientDo, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		return err
	}

	h.uprobes = append(h.uprobes, up)

	retOffsets, err := target.GetFunctionReturns(h.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := exec.Uprobe("", h.bpfObjects.UprobeHttpClientDoReturns, &link.UprobeOptions{
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
	var event Event
	for {
		record, err := h.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			h.logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			h.logger.Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			h.logger.Error(err, "error parsing perf event")
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
	h.logger.Info("closing net/http/client instrumentor")
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
