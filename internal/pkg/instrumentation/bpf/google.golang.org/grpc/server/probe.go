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
	"fmt"
	"os"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/events"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in the gRPC server during a gRPC request.
type Event struct {
	context.BaseSpanProperties
	Method [100]byte
}

// Instrumentor is the gRPC server instrumentor.
type Instrumentor struct {
	logger       logr.Logger
	bpfObjects   *bpfObjects
	uprobe       link.Link
	returnProbs  []link.Link
	headersProbe link.Link
	eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New(logger logr.Logger) *Instrumentor {
	return &Instrumentor{logger: logger.WithName("Instrumentor/GRPC/Server")}
}

// LibraryName returns the gRPC server package import path.
func (g *Instrumentor) LibraryName() string {
	return "google.golang.org/grpc/server"
}

// FuncNames returns the function names from "google.golang.org/grpc" that are
// instrumented.
func (g *Instrumentor) FuncNames() []string {
	return []string{
		"google.golang.org/grpc.(*Server).handleStream",
		"google.golang.org/grpc/internal/transport.(*http2Server).operateHeaders",
	}
}

// Load loads all instrumentation offsets.
func (g *Instrumentor) Load(exec *link.Executable, target *process.TargetDetails) error {
	targetLib := "google.golang.org/grpc"
	v := target.Libraries[targetLib]
	ver, err := version.NewVersion(v)
	if err != nil {
		return fmt.Errorf("invalid package version: %w", err)
	}

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
		inject.WithOffset("stream_method_ptr_pos", "google.golang.org/grpc/internal/transport.Stream", "method", ver),
		inject.WithOffset("stream_id_pos", "google.golang.org/grpc/internal/transport.Stream", "id", ver),
		inject.WithOffset("stream_ctx_pos", "google.golang.org/grpc/internal/transport.Stream", "ctx", ver),
		inject.WithOffset("frame_fields_pos", "golang.org/x/net/http2.MetaHeadersFrame", "Fields", ver),
		inject.WithOffset("frame_stream_id_pod", "golang.org/x/net/http2.FrameHeader", "StreamID", ver),
	)
	if err != nil {
		return err
	}

	g.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(target),
		},
	})
	if err != nil {
		return err
	}

	offset, err := target.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}

	up, err := exec.Uprobe("", g.bpfObjects.UprobeServerHandleStream, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		return err
	}

	g.uprobe = up
	retOffsets, err := target.GetFunctionReturns(g.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := exec.Uprobe("", g.bpfObjects.UprobeServerHandleStreamReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		g.returnProbs = append(g.returnProbs, retProbe)
	}

	headerOffset, err := target.GetFunctionOffset(g.FuncNames()[1])
	if err != nil {
		return err
	}
	hProbe, err := exec.Uprobe("", g.bpfObjects.UprobeDecodeStateDecodeHeader, &link.UprobeOptions{
		Address: headerOffset,
	})
	if err != nil {
		return err
	}
	g.headersProbe = hProbe

	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (g *Instrumentor) Run(eventsChan chan<- *events.Event) {
	var event Event
	for {
		record, err := g.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			g.logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			g.logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			g.logger.Error(err, "error parsing perf event")
			continue
		}

		eventsChan <- g.convertEvent(&event)
	}
}

func (g *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])

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
		Library:   g.LibraryName(),
		Name:      method,
		Kind:      trace.SpanKindServer,
		StartTime: int64(e.StartTime),
		EndTime:   int64(e.EndTime),
		Attributes: []attribute.KeyValue{
			semconv.RPCSystemKey.String("grpc"),
			semconv.RPCServiceKey.String(method),
		},
		ParentSpanContext: pscPtr,
		SpanContext:       &sc,
	}
}

// Close stops the Instrumentor.
func (g *Instrumentor) Close() {
	g.logger.V(0).Info("closing gRPC server instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	if g.uprobe != nil {
		g.uprobe.Close()
	}

	for _, r := range g.returnProbs {
		r.Close()
	}

	if g.headersProbe != nil {
		g.headersProbe.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
