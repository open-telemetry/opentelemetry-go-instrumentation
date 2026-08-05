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
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/kernel"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/pdataconv"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
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

// headerOffsetConst is a [probe.Const] for a struct field offset that more than
// one struct declares. The offset is resolved from the first ID with a valid
// offset for the instrumented version of grpc.
//
// grpc 1.82.1 split internal/transport.headerFrame into clientHeaders and
// serverHeaders. Both declare streamID and hf identically, so the offsets are
// interchangeable and only the struct they are read from differs. The struct is
// selected from the recorded offsets, not a version boundary: 1.83.0-dev and
// 1.84.0-dev sort above 1.82.1 but still declare headerFrame.
type headerOffsetConst struct {
	Key string
	IDs []structfield.ID

	logger *slog.Logger
}

// SetLogger sets the Logger for headerOffsetConst operations.
func (c headerOffsetConst) SetLogger(l *slog.Logger) probe.Const {
	c.logger = l
	return c
}

// StructFieldIDs returns the struct field IDs an offset is resolved from.
func (c headerOffsetConst) StructFieldIDs() []structfield.ID {
	return c.IDs
}

// InjectOption returns the [inject.Option] for the first ID with a valid
// offset for the version of grpc used. If no ID has one, an error is returned.
func (c headerOffsetConst) InjectOption(info *process.Info) (inject.Option, error) {
	ver, ok := info.Modules[pkg]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", pkg)
	}

	for _, id := range c.IDs {
		if off, ok := inject.GetOffset(id, ver); ok && off.Valid {
			if c.logger != nil {
				c.logger.Debug("Offset found", "key", c.Key, "id", id, "offset", off.Offset)
			}
			return inject.WithKeyValue(c.Key, off.Offset), nil
		}
	}

	// No cached offset for any ID. Fall back to analyzing the binary itself.
	for _, id := range c.IDs {
		if c.logger != nil {
			c.logger.Info("Offset not cached, analyzing directly", "key", c.Key, "id", id)
		}

		off, err := inject.FindOffset(id, info)
		if err == nil && off.Valid {
			return inject.WithKeyValue(c.Key, off.Offset), nil
		}
	}

	return nil, fmt.Errorf("failed to find valid offset for %q in any of %v", c.Key, c.IDs)
}

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
					ID: structfield.NewID(
						"google.golang.org/grpc",
						"google.golang.org/grpc",
						"ClientConn",
						"target",
					),
				},
				probe.StructFieldConst{
					Key: "httpclient_nextid_pos",
					ID: structfield.NewID(
						"google.golang.org/grpc",
						"google.golang.org/grpc/internal/transport",
						"http2Client",
						"nextID",
					),
				},
				headerOffsetConst{
					Key: "headerFrame_hf_pos",
					IDs: []structfield.ID{
						structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/transport",
							"headerFrame",
							"hf",
						),
						structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/transport",
							"clientHeaders",
							"hf",
						),
					},
				},
				headerOffsetConst{
					Key: "headerFrame_streamid_pos",
					IDs: []structfield.ID{
						structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/transport",
							"headerFrame",
							"streamID",
						),
						structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/transport",
							"clientHeaders",
							"streamID",
						),
					},
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "error_status_pos",
						ID: structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/status",
							"Error",
							"s",
						),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_s_pos",
						ID: structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/grpc/internal/status",
							"Status",
							"s",
						),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_code_pos",
						ID: structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/genproto/googleapis/rpc/status",
							"Status",
							"Code",
						),
					},
					MinVersion: writeStatusMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "status_message_pos",
						ID: structfield.NewID(
							"google.golang.org/grpc",
							"google.golang.org/genproto/googleapis/rpc/status",
							"Status",
							"Message",
						),
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
				// grpc 1.82.1 split (*loopyWriter).headerHandler into
				// clientHeaderHandler and serverHeaderHandler. Exactly one of
				// these symbols is present in any given binary, so the one that
				// does not resolve is ignored rather than failing the load.
				{
					Sym:         "google.golang.org/grpc/internal/transport.(*loopyWriter).headerHandler",
					EntryProbe:  "uprobe_LoopyWriter_HeaderHandler",
					FailureMode: probe.FailureModeIgnore,
				},
				{
					Sym:         "google.golang.org/grpc/internal/transport.(*loopyWriter).clientHeaderHandler",
					EntryProbe:  "uprobe_LoopyWriter_HeaderHandler",
					FailureMode: probe.FailureModeIgnore,
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
	if !kernel.SupportsContextPropagation() {
		return nil, errors.New(
			"the Linux Kernel doesn't support context propagation, please check if the kernel is in lockdown mode (/sys/kernel/security/lockdown)",
		)
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
		attrs = append(
			attrs,
			semconv.NetworkPeerPort(port),
			semconv.ServerPort(port),
		)
	}

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName(method)
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(kernel.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(kernel.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	pdataconv.Attributes(span.Attributes(), attrs...)

	if writeStatus && e.StatusCode > 0 {
		span.Status().SetCode(ptrace.StatusCodeError)
		errMsg := unix.ByteSliceToString(e.ErrMsg[:])
		if errMsg != "" {
			span.Status().SetMessage(errMsg)
		}
	}

	return spans
}
