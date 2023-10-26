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

	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "net/http"

// Event represents an event in an HTTP server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	Method [7]byte
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
	return &Instrumentor{logger: logger.WithName("Instrumentor/HTTP/Server")}
}

// LibraryName returns the net/http package name.
func (h *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "net/http" that are instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"net/http.HandlerFunc.ServeHTTP"}
}

// Load loads all instrumentation offsets.
func (h *Instrumentor) Load(exec *link.Executable, target *process.TargetDetails) error {
	ver := target.GoVersion

	spec, err := loadBpf()
	if err != nil {
		return err
	}
	err = inject.Constants(
		spec,
		inject.WithRegistersABI(target.IsRegistersABI()),
		inject.WithOffset("method_ptr_pos", structfield.NewID("net/http", "Request", "Method"), ver),
		inject.WithOffset("url_ptr_pos", structfield.NewID("net/http", "Request", "URL"), ver),
		inject.WithOffset("ctx_ptr_pos", structfield.NewID("net/http", "Request", "ctx"), ver),
		inject.WithOffset("path_ptr_pos", structfield.NewID("net/url", "URL", "Path"), ver),
		inject.WithOffset("ctx_ptr_pos", structfield.NewID("net/http", "Request", "ctx"), ver),
		inject.WithOffset("headers_ptr_pos", structfield.NewID("net/http", "Request", "Header"), ver),
		inject.WithOffset("buckets_ptr_pos", structfield.NewID("runtime", "hmap", "buckets"), ver),
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

	for _, funcName := range h.FuncNames() {
		h.registerProbes(exec, target, funcName)
	}

	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	h.eventsReader = rd

	return nil
}

func (h *Instrumentor) registerProbes(exec *link.Executable, target *process.TargetDetails, funcName string) {
	logger := h.logger.WithValues("function", funcName)
	offset, err := target.GetFunctionOffset(funcName)
	if err != nil {
		logger.Error(err, "could not find function start offset. Skipping")
		return
	}
	retOffsets, err := target.GetFunctionReturns(funcName)
	if err != nil {
		logger.Error(err, "could not find function end offsets. Skipping")
		return
	}

	up, err := exec.Uprobe("", h.bpfObjects.UprobeHandlerFuncServeHTTP, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.V(1).Info("could not insert start uprobe. Skipping",
			"error", err.Error())
		return
	}

	h.uprobes = append(h.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := exec.Uprobe("", h.bpfObjects.UprobeHandlerFuncServeHTTP_Returns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			logger.Error(err, "could not insert return uprobe. Skipping")
			return
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}
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
		},
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	h.logger.Info("closing net/http instrumentor")
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
