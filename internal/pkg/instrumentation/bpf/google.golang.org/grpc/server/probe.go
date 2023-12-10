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
	"os"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// name is the instrumentation name.
	name = "google.golang.org/grpc/server"
	// pkg is the package being instrumented.
	pkg = "google.golang.org/grpc"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	return &probe.Base[bpfObjects, event]{
		Name:            name,
		Logger:          logger.WithName(name),
		InstrumentedPkg: pkg,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "stream_method_ptr_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "method"),
			},
			probe.StructFieldConst{
				Key: "stream_id_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "id"),
			},
			probe.StructFieldConst{
				Key: "stream_ctx_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "ctx"),
			},
			probe.StructFieldConst{
				Key: "frame_fields_pos",
				Val: structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "MetaHeadersFrame", "Fields"),
			},
			probe.StructFieldConst{
				Key: "frame_stream_id_pod",
				Val: structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "FrameHeader", "StreamID"),
			},
		},
		Uprobes: []probe.Uprobe[bpfObjects]{
			{
				Sym: "google.golang.org/grpc.(*Server).handleStream",
				Fn:  uprobeHandleStream,
			},
			{
				Sym: "google.golang.org/grpc/internal/transport.(*http2Server).operateHeaders",
				Fn:  uprobeOperateHeaders,
			},
		},

		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize())
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

func uprobeHandleStream(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeServerHandleStream, opts)
	if err != nil {
		return nil, err
	}
	links := []link.Link{l}

	retOffsets, err := target.GetFunctionReturns(name)
	if err != nil {
		return nil, err
	}

	for _, ret := range retOffsets {
		opts := &link.UprobeOptions{Address: ret}
		l, err := exec.Uprobe("", obj.UprobeServerHandleStreamReturns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

func uprobeOperateHeaders(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeHttp2ServerOperateHeader, opts)
	if err != nil {
		return nil, err
	}
	return []link.Link{l}, nil
}

// event represents an event in the gRPC server during a gRPC request.
type event struct {
	context.BaseSpanProperties
	Method [100]byte
}

func convertEvent(e *event) *probe.Event {
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

	return &probe.Event{
		Package:   pkg,
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
