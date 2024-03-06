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

package grpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "google.golang.org/grpc"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger.WithName(id.String()),
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "clientconn_target_ptr_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc", "ClientConn", "target"),
			},
			probe.StructFieldConst{
				Key: "httpclient_nextid_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "http2Client", "nextID"),
			},
			probe.StructFieldConst{
				Key: "headerFrame_hf_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "hf"),
			},
			probe.StructFieldConst{
				Key: "headerFrame_streamid_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "streamID"),
			},
		},
		Uprobes: []probe.Uprobe[bpfObjects]{
			{
				Sym: "google.golang.org/grpc.(*ClientConn).Invoke",
				Fn:  uprobeInvoke,
			}, {
				Sym: "google.golang.org/grpc/internal/transport.(*http2Client).NewStream",
				Fn: func(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
					prog := obj.UprobeHttp2ClientNewStream
					return uprobeFn(name, exec, target, prog)
				},
			}, {
				Sym: "google.golang.org/grpc/internal/transport.(*loopyWriter).headerHandler",
				Fn: func(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
					prog := obj.UprobeLoopyWriterHeaderHandler
					return uprobeFn(name, exec, target, prog)
				},
			},
		},

		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize())
		},
		SpecFn:    verifyAndLoadBpf,
		ProcessFn: convertEvent,
	}
}

func verifyAndLoadBpf() (*ebpf.CollectionSpec, error) {
	if !utils.SupportsContextPropagation() {
		return nil, fmt.Errorf("the Linux Kernel doesn't support context propagation, please check if the kernel is in lockdown mode (/sys/kernel/security/lockdown)")
	}

	return loadBpf()
}

func uprobeFn(name string, exec *link.Executable, target *process.TargetDetails, prog *ebpf.Program) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", prog, opts)
	if err != nil {
		return nil, err
	}
	return []link.Link{l}, nil
}

func uprobeInvoke(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	links, err := uprobeFn(name, exec, target, obj.UprobeClientConnInvoke)
	if err != nil {
		return nil, err
	}

	retOffsets, err := target.GetFunctionReturns(name)
	if err != nil {
		return nil, err
	}

	for _, ret := range retOffsets {
		opts := &link.UprobeOptions{Address: ret}
		l, err := exec.Uprobe("", obj.UprobeClientConnInvokeReturns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

// event represents an event in the gRPC client during a gRPC request.
type event struct {
	context.BaseSpanProperties
	Method [50]byte
	Target [50]byte
}

// According to https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func convertEvent(e *event) *probe.SpanEvent {
	method := unix.ByteSliceToString(e.Method[:])
	target := unix.ByteSliceToString(e.Target[:])
	var attrs []attribute.KeyValue

	// remove port
	if parts := strings.Split(target, ":"); len(parts) > 1 {
		target = parts[0]
		if remotePeerPortInt, err := strconv.Atoi(parts[1]); err == nil {
			attrs = append(attrs, semconv.NetworkPeerPort(remotePeerPortInt))
		}
	}

	attrs = append(attrs, semconv.RPCSystemKey.String("grpc"),
		semconv.RPCServiceKey.String(method),
		semconv.NetPeerNameKey.String(target))

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

	return &probe.SpanEvent{
		SpanName:          method,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		Attributes:        attrs,
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
	}
}
