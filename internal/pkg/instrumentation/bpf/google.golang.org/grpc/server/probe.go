// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "google.golang.org/grpc"

	// writeStatusMin is the minimum version of grpc that supports status
	// parsing.
	writeStatusMin = "1.40.0"

	// writeStatusV2 is the version the writeStatus method changed.
	writeStatusV2 = "1.69.0"
)

var (
	writeStatusMinVersion = version.Must(version.NewVersion(writeStatusMin))
	writeStatusV2Version  = version.Must(version.NewVersion(writeStatusV2))
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger, ver string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindServer,
		InstrumentedPkg: pkg,
	}
	p := &processor{Logger: logger}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.RegistersABIConst{},
				probe.AllocationConst{},
				serverAddrConst{},
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
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "server_stream_stream_pos",
						Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "ServerStream", "Stream"),
					},
					MinVersion: writeStatusV2Version,
				},
				probe.StructFieldConst{
					Key: "frame_fields_pos",
					Val: structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "MetaHeadersFrame", "Fields"),
				},
				probe.StructFieldConst{
					Key: "frame_stream_id_pod",
					Val: structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "FrameHeader", "StreamID"),
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_s_pos",
						Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Status", "s"),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_code_pos",
						Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/genproto/googleapis/rpc/status", "Status", "Code"),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "http2server_peer_pos",
						Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "http2Server", "peer"),
					},
					MinVersion: serverAddrMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "peer_local_addr_pos",
						Val: structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/peer", "Peer", "LocalAddr"),
					},
					MinVersion: serverAddrMinVersion,
				},
				probe.StructFieldConst{
					Key: "TCPAddr_IP_offset",
					Val: structfield.NewID("std", "net", "TCPAddr", "IP"),
				},
				probe.StructFieldConst{
					Key: "TCPAddr_Port_offset",
					Val: structfield.NewID("std", "net", "TCPAddr", "Port"),
				},
				framePosConst{},
			},
			Uprobes: []probe.Uprobe{
				{
					Sym:         "google.golang.org/grpc.(*Server).handleStream",
					EntryProbe:  "uprobe_server_handleStream",
					ReturnProbe: "uprobe_server_handleStream_Returns",
					PackageConstrainsts: []probe.PackageConstrainst{
						{
							Package: "google.golang.org/grpc",
							Constraints: version.MustConstraints(
								version.NewConstraint(fmt.Sprintf(
									"> %s, < %s", writeStatusMin, writeStatusV2),
								),
							),
							FailureMode: probe.FailureModeIgnore,
						},
					},
				},
				{
					Sym:         "google.golang.org/grpc.(*Server).handleStream",
					EntryProbe:  "uprobe_server_handleStream2",
					ReturnProbe: "uprobe_server_handleStream2_Returns",
					PackageConstrainsts: []probe.PackageConstrainst{
						{
							Package: "google.golang.org/grpc",
							Constraints: version.MustConstraints(
								version.NewConstraint(">= 1.69.0"),
							),
							FailureMode: probe.FailureModeIgnore,
						},
					},
				},
				{
					Sym:        "google.golang.org/grpc/internal/transport.(*http2Server).operateHeaders",
					EntryProbe: "uprobe_http2Server_operateHeader",
				},
				{
					Sym:        "google.golang.org/grpc/internal/transport.(*http2Server).WriteStatus",
					EntryProbe: "uprobe_http2Server_WriteStatus",
					PackageConstrainsts: []probe.PackageConstrainst{
						{
							Package: "google.golang.org/grpc",
							Constraints: version.MustConstraints(
								version.NewConstraint(fmt.Sprintf(
									"> %s, < %s", writeStatusMin, writeStatusV2),
								),
							),
							FailureMode: probe.FailureModeIgnore,
						},
					},
				},
				{
					Sym:        "google.golang.org/grpc/internal/transport.(*http2Server).writeStatus",
					EntryProbe: "uprobe_http2Server_WriteStatus2",
					PackageConstrainsts: []probe.PackageConstrainst{
						{
							Package: "google.golang.org/grpc",
							Constraints: version.MustConstraints(
								version.NewConstraint(">= 1.69.0"),
							),
							FailureMode: probe.FailureModeIgnore,
						},
					},
				},
			},
			SpecFn: loadBpf,
		},
		Version:   ver,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: p.processFn,
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

type serverAddrConst struct{}

var (
	serverAddrMinVersion = version.Must(version.NewVersion("1.60.0"))
	serverAddr           = false
)

func (w serverAddrConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	ver, ok := td.Libraries[pkg]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", pkg)
	}
	if ver.GreaterThanOrEqual(serverAddrMinVersion) {
		serverAddr = true
	}
	return inject.WithKeyValue("server_addr_supported", serverAddr), nil
}

// event represents an event in the gRPC server during a gRPC request.
type event struct {
	context.BaseSpanProperties
	Method     [100]byte
	StatusCode int32
	LocalAddr  NetAddr
	HasStatus  uint8
}

type NetAddr struct {
	IP   [16]uint8
	Port int32
}

type processor struct {
	Logger *slog.Logger
}

func (p *processor) processFn(e *event) ptrace.SpanSlice {
	p.Logger.Debug("processing event", "event", e)
	method := unix.ByteSliceToString(e.Method[:])

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName(method)
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	attrs := []attribute.KeyValue{
		semconv.RPCSystemKey.String("grpc"),
		semconv.RPCServiceKey.String(method),
		semconv.RPCGRPCStatusCodeKey.Int(int(e.StatusCode)),
	}

	if e.HasStatus != 0 {
		attrs = append(attrs, semconv.RPCGRPCStatusCodeKey.Int(int(e.StatusCode)))

		// Set server status codes per semconv:
		// https://github.com/open-telemetry/semantic-conventions/blob/02ecf0c71e9fa74d09d81c48e04a132db2b7060b/docs/rpc/grpc.md#grpc-status
		switch e.StatusCode {
		case int32(codes.Unknown), int32(codes.DeadlineExceeded),
			int32(codes.Unimplemented), int32(codes.Internal),
			int32(codes.Unavailable), int32(codes.DataLoss):
			span.Status().SetCode(ptrace.StatusCodeError)
		}
	}

	if serverAddr {
		attrs = append(attrs, semconv.ServerAddress(net.IP(e.LocalAddr.IP[:]).String()))
		attrs = append(attrs, semconv.ServerPort(int(e.LocalAddr.Port)))
	}

	utils.Attributes(span.Attributes(), attrs...)

	return spans
}
