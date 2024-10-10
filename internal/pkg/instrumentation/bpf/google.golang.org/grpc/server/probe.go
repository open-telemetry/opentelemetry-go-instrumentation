// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"log/slog"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

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
func New(logger *slog.Logger, version string) probe.Probe {
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
		},
		SpecFn:    loadBpf,
		ProcessFn: processFn(pkg, version, semconv.SchemaURL),
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
	Method [100]byte
}

func processFn(pkg, ver, schemaURL string) func(*event) ptrace.ScopeSpans {
	scopeName := "go.opentelemetry.io/auto/" + pkg
	return func(e *event) ptrace.ScopeSpans {
		ss := ptrace.NewScopeSpans()

		scope := ss.Scope()
		scope.SetName(scopeName)
		scope.SetVersion(ver)
		ss.SetSchemaUrl(schemaURL)

		method := unix.ByteSliceToString(e.Method[:])

		span := ss.Spans().AppendEmpty()
		span.SetName(method)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
		span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
		span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
		span.SetFlags(uint32(trace.FlagsSampled))

		if e.ParentSpanContext.SpanID.IsValid() {
			span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
		}

		utils.Attributes(
			span.Attributes(),
			semconv.RPCSystemKey.String("grpc"),
			semconv.RPCServiceKey.String(method),
		)

		return ss
	}
}
