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
	"net"
	"os"
	"strconv"

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
	name = "net/http/server"
	// pkg is the package being instrumented.
	pkg = "net/http"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	return &probe.Base[bpfObjects, event]{
		Name:            name,
		Logger:          logger.WithName(name),
		InstrumentedPkg: pkg,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.StructFieldConst{
				Key: "method_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Method"),
			},
			probe.StructFieldConst{
				Key: "url_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "URL"),
			},
			probe.StructFieldConst{
				Key: "ctx_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "ctx"),
			},
			probe.StructFieldConst{
				Key: "path_ptr_pos",
				Val: structfield.NewID("std", "net/url", "URL", "Path"),
			},
			probe.StructFieldConst{
				Key: "headers_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Header"),
			},
			probe.StructFieldConst{
				Key: "req_ptr_pos",
				Val: structfield.NewID("std", "net/http", "response", "req"),
			},
			probe.StructFieldConst{
				Key: "status_code_pos",
				Val: structfield.NewID("std", "net/http", "response", "status"),
			},
			probe.StructFieldConst{
				Key: "buckets_ptr_pos",
				Val: structfield.NewID("std", "runtime", "hmap", "buckets"),
			},
			probe.StructFieldConst{
				Key: "remote_addr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "RemoteAddr"),
			},
			probe.StructFieldConst{
				Key: "host_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Host"),
			},
			probe.StructFieldConst{
				Key: "proto_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Proto"),
			},
		},
		Uprobes: map[string]probe.UprobeFunc[bpfObjects]{
			"net/http.serverHandler.ServeHTTP": uprobeServeHTTP,
		},

		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize())
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

func uprobeServeHTTP(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeHandlerFuncServeHTTP, opts)
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
		l, err := exec.Uprobe("", obj.UprobeHandlerFuncServeHTTP_Returns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

// event represents an event in an HTTP server during an HTTP
// request-response.
type event struct {
	context.BaseSpanProperties
	StatusCode uint64
	Method     [8]byte
	Path       [128]byte
	RemoteAddr [256]byte
	Host       [256]byte
	Proto      [8]byte
}

func convertEvent(e *event) *probe.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])
	remoteAddr := unix.ByteSliceToString(e.RemoteAddr[:])
	host := unix.ByteSliceToString(e.Host[:])
	proto := unix.ByteSliceToString(e.Proto[:])
	ip, port, isRemoteAddrValid := net.SplitHostPort(remoteAddr)

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

	attributes := []attribute.KeyValue{
		semconv.HTTPMethodKey.String(method),
		semconv.HTTPTargetKey.String(path),
		semconv.HTTPResponseStatusCodeKey.Int(int(e.StatusCode)),
		semconv.NetHostName(host),
		semconv.NetProtocolName(proto),
	}

	if isRemoteAddrValid == nil {
		remotePeerAddr, remotePeerPort := ip, port

		attributes = append(attributes, semconv.NetPeerName(remotePeerAddr))

		if remotePeerPortInt, err := strconv.Atoi(remotePeerPort); err == nil {
			attributes = append(attributes, semconv.NetPeerPort(remotePeerPortInt))
		}
	}

	return &probe.Event{
		Package: pkg,
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference).
		Name:              method,
		Kind:              trace.SpanKindServer,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		Attributes:        attributes,
	}
}
