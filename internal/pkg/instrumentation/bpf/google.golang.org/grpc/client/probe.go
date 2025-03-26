// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package grpc provides an instrumentation probe for [google.golang.org/grpc]
// clients.
package grpc

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

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
)

var (
	writeStatus           = false
	writeStatusMinVersion = semver.New(1, 40, 0, "", "")
)

type writeStatusConst struct{}

func (w writeStatusConst) InjectOption(info *process.Info) (inject.Option, error) {
	ver, ok := info.Modules[pkg]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", pkg)
	}
	if ver.GreaterThanEqual(writeStatusMinVersion) {
		writeStatus = true
	}
	return inject.WithKeyValue("write_status_supported", writeStatus), nil
}

// New returns a new [probe.Probe].
func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.AllocationConst{},
				writeStatusConst{},
				probe.StructFieldConst{
					Key: "clientconn_target_ptr_pos",
					ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc", "ClientConn", "target"),
				},
				probe.StructFieldConst{
					Key: "httpclient_nextid_pos",
					ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "http2Client", "nextID"),
				},
				probe.StructFieldConst{
					Key: "headerFrame_hf_pos",
					ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "hf"),
				},
				probe.StructFieldConst{
					Key: "headerFrame_streamid_pos",
					ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "streamID"),
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "error_status_pos",
						ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Error", "s"),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_s_pos",
						ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/status", "Status", "s"),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_code_pos",
						ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/genproto/googleapis/rpc/status", "Status", "Code"),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_message_pos",
						ID:  structfield.NewID("google.golang.org/grpc", "google.golang.org/genproto/googleapis/rpc/status", "Status", "Message"),
					},
					MinVersion: writeStatusMinVersion,
				},
			},
			Uprobes: []*probe.Uprobe{
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
			SpecFn: verifyAndLoadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

func verifyAndLoadBpf() (*ebpf.CollectionSpec, error) {
	if !utils.SupportsContextPropagation() {
		return nil, errors.New("the Linux Kernel doesn't support context propagation, please check if the kernel is in lockdown mode (/sys/kernel/security/lockdown)")
	}

	return loadBpf()
}

// event represents an event in the gRPC client during a gRPC request.
type event struct {
	context.BaseSpanProperties
	ErrMsg     [128]byte
	Method     [50]byte
	Target     [50]byte
	StatusCode int32
}

func processFn(e *event) ptrace.SpanSlice {
	method := unix.ByteSliceToString(e.Method[:])
	address := unix.ByteSliceToString(e.Target[:])

	var port int
	host, portStr, err := net.SplitHostPort(address)
	if err == nil {
		port, _ = strconv.Atoi(portStr)
	} else {
		host = address
	}

	attrs := []attribute.KeyValue{
		semconv.RPCSystemKey.String("grpc"),
		semconv.RPCServiceKey.String(method),
		semconv.ServerAddress(host),
		semconv.RPCGRPCStatusCodeKey.Int(int(e.StatusCode)),
	}

	if port > 0 {
		attrs = append(attrs, semconv.NetworkPeerPort(port))
		attrs = append(attrs, semconv.ServerPort(port))
	}

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName(method)
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	utils.Attributes(span.Attributes(), attrs...)

	if writeStatus && e.StatusCode > 0 {
		span.Status().SetCode(ptrace.StatusCodeError)
		errMsg := unix.ByteSliceToString(e.ErrMsg[:])
		if errMsg != "" {
			span.Status().SetMessage(errMsg)
		}
	}

	return spans
}
