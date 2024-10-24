// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"log/slog"

	"github.com/hashicorp/go-version"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
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
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindServer,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
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
			probe.StructFieldConst{
				Key: "status_s_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Status", "s"),
			},
			probe.StructFieldConst{
				Key: "status_code_pos",
				Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/genproto/googleapis/rpc/status", "Status", "Code"),
			},
			framePosConst{},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:         "google.golang.org/grpc.(*Server).handleStream",
				EntryProbe:  "uprobe_server_handleStream",
				ReturnProbe: "uprobe_server_handleStream_Returns",
			},
			{
				Sym:        "google.golang.org/grpc/internal/transport.(*http2Server).operateHeaders",
				EntryProbe: "uprobe_http2Server_operateHeader",
			},
			{
				Sym:         "google.golang.org/grpc/internal/transport.(*http2Server).WriteStatus",
				EntryProbe:  "uprobe_http2Server_WriteStatus",
			},
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

// framePosConst is a Probe Const defining the position of the
// http.MetaHeadersFrame parameter of the http2Server.operateHeaders method.
type framePosConst struct{}

// Prior to v1.60.0 the frame parameter was first. However, in that version a
// context was added as the first parameter. The frame became the second
// parameter:
// https://github.com/grpc/grpc-go/pull/6716/files#diff-4058722211b8d52e2d5b0c0b7542059ed447a04017b69520d767e94a9493409eR334
var paramChangeVer = version.Must(version.NewVersion("1.60.0"))

func (c framePosConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	ver, ok := td.Libraries[pkg]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", pkg)
	}

	return inject.WithKeyValue("is_new_frame_pos", ver.GreaterThanOrEqual(paramChangeVer)), nil
}

// event represents an event in the gRPC server during a gRPC request.
type event struct {
	context.BaseSpanProperties
	Method     [100]byte
	StatusCode int32
}

func convertEvent(e *event) []*probe.SpanEvent {
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

	event := &probe.SpanEvent{
		SpanName:  method,
		StartTime: utils.BootOffsetToTime(e.StartTime),
		EndTime:   utils.BootOffsetToTime(e.EndTime),
		Attributes: []attribute.KeyValue{
			semconv.RPCSystemKey.String("grpc"),
			semconv.RPCServiceKey.String(method),
			semconv.RPCGRPCStatusCodeKey.Int(int(e.StatusCode)),
		},
		ParentSpanContext: pscPtr,
		SpanContext:       &sc,
		TracerSchema:      semconv.SchemaURL,
	}

	// Set server status codes per semconv:
	// See https://github.com/open-telemetry/semantic-conventions/blob/02ecf0c71e9fa74d09d81c48e04a132db2b7060b/docs/rpc/grpc.md#grpc-status
	if e.StatusCode == int32(codes.Unknown) ||
		e.StatusCode == int32(codes.DeadlineExceeded) ||
		e.StatusCode == int32(codes.Unimplemented) ||
		e.StatusCode == int32(codes.Internal) ||
		e.StatusCode == int32(codes.Unavailable) ||
		e.StatusCode == int32(codes.DataLoss) {
		event.Status = probe.Status{Code: otelcodes.Error}
	}
	return []*probe.SpanEvent{event}
}
