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

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "net/http"

// Event represents an event in an HTTP server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	StatusCode uint64
	Method     [8]byte
	Path       [128]byte
}

// Probe is the net/http instrumentation probe.
type Probe struct {
	logger       logr.Logger
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Probe].
func New(logger logr.Logger) *Probe {
	return &Probe{logger: logger.WithName("Probe/HTTP/Server")}
}

// LibraryName returns the net/http package name.
func (h *Probe) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "net/http" that are instrumented.
func (h *Probe) FuncNames() []string {
	return []string{"net/http.HandlerFunc.ServeHTTP"}
}

// Load loads all instrumentation offsets.
func (h *Probe) Load(exec *link.Executable, target *process.TargetDetails) error {
	ver := target.GoVersion

	spec, err := loadBpf()
	if err != nil {
		return err
	}
	err = inject.Constants(
		spec,
		inject.WithRegistersABI(target.IsRegistersABI()),
		inject.WithOffset("method_ptr_pos", structfield.NewID("std", "net/http", "Request", "Method"), ver),
		inject.WithOffset("url_ptr_pos", structfield.NewID("std", "net/http", "Request", "URL"), ver),
		inject.WithOffset("ctx_ptr_pos", structfield.NewID("std", "net/http", "Request", "ctx"), ver),
		inject.WithOffset("path_ptr_pos", structfield.NewID("std", "net/url", "URL", "Path"), ver),
		inject.WithOffset("headers_ptr_pos", structfield.NewID("std", "net/http", "Request", "Header"), ver),
		inject.WithOffset("req_ptr_pos", structfield.NewID("std", "net/http", "response", "req"), ver),
		inject.WithOffset("status_code_pos", structfield.NewID("std", "net/http", "response", "status"), ver),
		inject.WithOffset("buckets_ptr_pos", structfield.NewID("std", "runtime", "hmap", "buckets"), ver),
	)
	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(target),
		},
	})
	if err != nil {
		return err
	}

	// TODO : crteate a function to register probes and handle different configurations of entry and exit probes
	// register entry and exit probes for net/http.HandlerFunc.ServeHTTP
	offset, err := target.GetFunctionOffset(h.FuncNames()[0])
	if err != nil {
		return err
	}

	up, err := exec.Uprobe("", h.bpfObjects.UprobeHandlerFuncServeHTTP, &link.UprobeOptions{
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
		retProbe, err := exec.Uprobe("", h.bpfObjects.UprobeHandlerFuncServeHTTP_Returns, &link.UprobeOptions{
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
func (h *Probe) Run(eventsChan chan<- *probe.Event) {
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

func (h *Probe) convertEvent(e *Event) *probe.Event {
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

	return &probe.Event{
		Library: h.LibraryName(),
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference).
		Name:              method,
		Kind:              trace.SpanKindServer,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
			semconv.HTTPStatusCodeKey.Int(int(e.StatusCode)),
		},
	}
}

// Close stops the Probe.
func (h *Probe) Close() {
	h.logger.Info("closing net/http probe")
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
