// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"log/slog"
	"strings"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "net/http"
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindServer,
		InstrumentedPkg: pkg,
	}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.RegistersABIConst{},
				probe.StructFieldConst{
					Key: "method_ptr_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "Method"),
				},
				probe.StructFieldConst{
					Key: "url_ptr_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "URL"),
				},
				probe.StructFieldConst{
					Key: "ctx_ptr_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "ctx"),
				},
				probe.StructFieldConst{
					Key: "path_ptr_pos",
					ID:  structfield.NewID("std", "net/url", "URL", "Path"),
				},
				probe.StructFieldConst{
					Key: "headers_ptr_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "Header"),
				},
				probe.StructFieldConst{
					Key: "req_ptr_pos",
					ID:  structfield.NewID("std", "net/http", "response", "req"),
				},
				probe.StructFieldConst{
					Key: "status_code_pos",
					ID:  structfield.NewID("std", "net/http", "response", "status"),
				},
				probe.StructFieldConst{
					Key: "buckets_ptr_pos",
					ID:  structfield.NewID("std", "runtime", "hmap", "buckets"),
				},
				probe.StructFieldConst{
					Key: "remote_addr_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "RemoteAddr"),
				},
				probe.StructFieldConst{
					Key: "host_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "Host"),
				},
				probe.StructFieldConst{
					Key: "proto_pos",
					ID:  structfield.NewID("std", "net/http", "Request", "Proto"),
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "req_pat_pos",
						ID:  structfield.NewID("std", "net/http", "Request", "pat"),
					},
					MinVersion: patternPathMinVersion,
				},
				probe.StructFieldConstMinVersion{
					StructField: probe.StructFieldConst{
						Key: "pat_str_pos",
						ID:  structfield.NewID("std", "net/http", "pattern", "str"),
					},
					MinVersion: patternPathMinVersion,
				},
				patternPathSupportedConst{},
			},
			Uprobes: []*probe.Uprobe{
				{
					Sym:         "net/http.serverHandler.ServeHTTP",
					EntryProbe:  "uprobe_serverHandler_ServeHTTP",
					ReturnProbe: "uprobe_serverHandler_ServeHTTP_Returns",
				},
			},
			SpecFn: loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

type patternPathSupportedConst struct{}

var (
	patternPathMinVersion  = version.Must(version.NewVersion("1.22.0"))
	isPatternPathSupported = false
)

func (c patternPathSupportedConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	isPatternPathSupported = td.GoVersion.GreaterThanOrEqual(patternPathMinVersion)
	return inject.WithKeyValue("pattern_path_supported", isPatternPathSupported), nil
}

// event represents an event in an HTTP server during an HTTP
// request-response.
type event struct {
	context.BaseSpanProperties
	StatusCode  uint64
	Method      [8]byte
	Path        [128]byte
	PathPattern [128]byte
	RemoteAddr  [256]byte
	Host        [256]byte
	Proto       [8]byte
}

func processFn(e *event) ptrace.SpanSlice {
	path := unix.ByteSliceToString(e.Path[:])
	method := unix.ByteSliceToString(e.Method[:])
	patternPath := unix.ByteSliceToString(e.PathPattern[:])

	isValidPatternPath := true
	patternPath, err := http.ParsePattern(patternPath)
	if err != nil || patternPath == "" {
		isValidPatternPath = false
	}

	proto := unix.ByteSliceToString(e.Proto[:])

	// https://www.rfc-editor.org/rfc/rfc9110.html#name-status-codes
	const maxStatus = 599
	if e.StatusCode > maxStatus {
		e.StatusCode = 0
	}
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLPath(path),
		semconv.HTTPResponseStatusCodeKey.Int(int(e.StatusCode)), // nolint: gosec  // Bound checked.
	}

	// Client address and port
	peerAddr, peerPort := http.NetPeerAddressPortAttributes(e.RemoteAddr[:])
	if peerAddr.Valid() {
		attrs = append(attrs, peerAddr)
	}
	if peerPort.Valid() {
		attrs = append(attrs, peerPort)
	}

	// Server address and port
	serverAddr, serverPort := http.ServerAddressPortAttributes(e.Host[:])
	if serverAddr.Valid() {
		attrs = append(attrs, serverAddr)
	}
	if serverPort.Valid() {
		attrs = append(attrs, serverPort)
	}

	if proto != "" {
		parts := strings.Split(proto, "/")
		if len(parts) == 2 {
			if parts[0] != "HTTP" {
				attrs = append(attrs, semconv.NetworkProtocolName(parts[0]))
			}
			attrs = append(attrs, semconv.NetworkProtocolVersion(parts[1]))
		}
	}

	spanName := method
	if isPatternPathSupported && isValidPatternPath {
		spanName = spanName + " " + patternPath
		attrs = append(attrs, semconv.HTTPRouteKey.String(patternPath))
	}

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName(spanName)
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	utils.Attributes(span.Attributes(), attrs...)

	if e.StatusCode >= 500 && e.StatusCode < 600 {
		span.Status().SetCode(ptrace.StatusCodeError)
	}

	return spans
}
