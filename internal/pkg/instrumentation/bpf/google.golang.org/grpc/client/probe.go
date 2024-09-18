// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "google.golang.org/grpc"
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
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
			probe.StructFieldConst{
				Key: "error_status_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Error", "s"),
			},
			probe.StructFieldConst{
				Key: "status_s_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Status", "s"),
			},
			probe.StructFieldConst{
				Key: "status_code_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/genproto/googleapis/rpc/status", "Status", "Code"),
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:         "google.golang.org/grpc.(*ClientConn).Invoke",
				EntryProbe:  "uprobe_ClientConn_Invoke",
				ReturnProbe: "uprobe_ClientConn_Invoke_Returns",
			},
			{
				Sym:        "google.golang.org/grpc/internal/transport.(*http2Client).NewStream",
				EntryProbe: "uprobe_http2Client_NewStream",
			},
			{
				Sym:        "google.golang.org/grpc/internal/transport.(*loopyWriter).headerHandler",
				EntryProbe: "uprobe_LoopyWriter_HeaderHandler",
			},
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

// event represents an event in the gRPC client during a gRPC request.
type event struct {
	context.BaseSpanProperties
	Method     [50]byte
	Target     [50]byte
	StatusCode int32
}

// According to https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func convertEvent(e *event) []*probe.SpanEvent {
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
		semconv.ServerAddress(target))

	attrs = append(attrs, semconv.RPCGRPCStatusCodeKey.Int(int(e.StatusCode)))

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

	event := &probe.SpanEvent{
		SpanName:          method,
		StartTime:         utils.BootOffsetToTime(e.StartTime),
		EndTime:           utils.BootOffsetToTime(e.EndTime),
		Attributes:        attrs,
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		TracerSchema:      semconv.SchemaURL,
	}

	if e.StatusCode > 0 {
		event.Status = probe.Status{Code: codes.Error}
	}

	return []*probe.SpanEvent{event}
}
