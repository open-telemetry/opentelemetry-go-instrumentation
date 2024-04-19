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

package client

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf_no_tp ./bpf/probe.bpf.c -- -DNO_HEADER_PROPAGATION

const (
	// pkg is the package being instrumented.
	pkg = "net/http"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}

	uprobes := []probe.Uprobe[bpfObjects]{
		{
			Sym: "net/http.(*Transport).roundTrip",
			Fn:  uprobeRoundTrip,
		},
	}

	// If the kernel supports context propagation, we enable the
	// probe which writes the data in the outgoing buffer.
	if utils.SupportsContextPropagation() {
		uprobes = append(uprobes,
			probe.Uprobe[bpfObjects]{
				Sym: "net/http.Header.writeSubset",
				Fn:  uprobeWriteSubset,
				// We mark this probe as dependent on roundTrip, so we don't accidentally
				// enable this bpf program, if the executable has compiled in writeSubset,
				// but doesn't have any http roundTrip.
				DependsOn: []string{"net/http.(*Transport).roundTrip"},
			},
		)
	}

	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger.WithName(id.String()),
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "method_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Method"),
			},
			probe.StructFieldConst{
				Key: "url_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "URL"),
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
				Key: "ctx_ptr_pos",
				Val: structfield.NewID("std", "net/http", "Request", "ctx"),
			},
			probe.StructFieldConst{
				Key: "status_code_pos",
				Val: structfield.NewID("std", "net/http", "Response", "StatusCode"),
			},
			probe.StructFieldConst{
				Key: "buckets_ptr_pos",
				Val: structfield.NewID("std", "runtime", "hmap", "buckets"),
			},
			probe.StructFieldConst{
				Key: "request_host_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Host"),
			},
			probe.StructFieldConst{
				Key: "request_proto_pos",
				Val: structfield.NewID("std", "net/http", "Request", "Proto"),
			},
			probe.StructFieldConst{
				Key: "io_writer_buf_ptr_pos",
				Val: structfield.NewID("std", "bufio", "Writer", "buf"),
			},
			probe.StructFieldConst{
				Key: "io_writer_n_pos",
				Val: structfield.NewID("std", "bufio", "Writer", "n"),
			},
			probe.StructFieldConst{
				Key: "scheme_pos",
				Val: structfield.NewID("std", "net/url", "URL", "Scheme"),
			},
			probe.StructFieldConst{
				Key: "opaque_pos",
				Val: structfield.NewID("std", "net/url", "URL", "Opaque"),
			},
			probe.StructFieldConst{
				Key: "user_ptr_pos",
				Val: structfield.NewID("std", "net/url", "URL", "User"),
			},
			probe.StructFieldConst{
				Key: "raw_path_pos",
				Val: structfield.NewID("std", "net/url", "URL", "RawPath"),
			},
			probe.StructFieldConst{
				Key: "omit_host_pos",
				Val: structfield.NewID("std", "net/url", "URL", "OmitHost"),
			},
			probe.StructFieldConst{
				Key: "force_query_pos",
				Val: structfield.NewID("std", "net/url", "URL", "ForceQuery"),
			},
			probe.StructFieldConst{
				Key: "raw_query_pos",
				Val: structfield.NewID("std", "net/url", "URL", "RawQuery"),
			},
			probe.StructFieldConst{
				Key: "fragment_pos",
				Val: structfield.NewID("std", "net/url", "URL", "Fragment"),
			},
			probe.StructFieldConst{
				Key: "raw_fragment_pos",
				Val: structfield.NewID("std", "net/url", "URL", "RawFragment"),
			},
			probe.StructFieldConst{
				Key: "username_pos",
				Val: structfield.NewID("std", "net/url", "Userinfo", "username"),
			},
		},
		Uprobes: uprobes,
		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize())
		},
		SpecFn:    verifyAndLoadBpf,
		ProcessFn: convertEvent,
	}
}

func verifyAndLoadBpf() (*ebpf.CollectionSpec, error) {
	if !utils.SupportsContextPropagation() {
		fmt.Fprintf(os.Stderr, "the Linux Kernel doesn't support context propagation, please check if the kernel is in lockdown mode (/sys/kernel/security/lockdown)")
		return loadBpf_no_tp()
	}

	return loadBpf()
}

func uprobeRoundTrip(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset, PID: target.PID}
	l, err := exec.Uprobe("", obj.UprobeTransportRoundTrip, opts)
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
		l, err := exec.Uprobe("", obj.UprobeTransportRoundTripReturns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

func uprobeWriteSubset(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeWriteSubset, opts)
	if err != nil {
		return nil, err
	}

	return []link.Link{l}, nil
}

// event represents an event in an HTTP server during an HTTP
// request-response.
type event struct {
	context.BaseSpanProperties
	Host        [128]byte
	Proto       [8]byte
	StatusCode  uint64
	Method      [10]byte
	Path        [100]byte
	Scheme      [8]byte
	Opaque      [8]byte
	RawPath     [8]byte
	Username    [8]byte
	RawQuery    [128]byte
	Fragment    [50]byte
	RawFragment [50]byte
	ForceQuery  uint8
	OmitHost    uint8
}

func convertEvent(e *event) []*probe.SpanEvent {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])
	scheme := unix.ByteSliceToString(e.Scheme[:])
	opaque := unix.ByteSliceToString(e.Opaque[:])
	host := unix.ByteSliceToString(e.Host[:])
	rawPath := unix.ByteSliceToString(e.RawPath[:])
	rawQuery := unix.ByteSliceToString(e.RawQuery[:])
	username := unix.ByteSliceToString(e.Username[:])
	fragment := unix.ByteSliceToString(e.Fragment[:])
	rawFragment := unix.ByteSliceToString(e.RawFragment[:])
	forceQuery := e.ForceQuery != 0
	omitHost := e.OmitHost != 0
	var user *url.Userinfo
	if len(username) > 0 {
		// check that username!="", otherwise url.User will instantiate
		// an empty, non-nil *Userinfo object which url.String() will parse
		// to just "@" in the final fullUrl
		user = url.User(username)
	}

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

	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPResponseStatusCodeKey.Int(int(e.StatusCode)),
	}

	if path != "" {
		attrs = append(attrs, semconv.URLPath(path))
	}

	url := &url.URL{
		Path:        path,
		Scheme:      scheme,
		Opaque:      opaque,
		Host:        host,
		RawPath:     rawPath,
		User:        user,
		RawQuery:    rawQuery,
		Fragment:    fragment,
		RawFragment: rawFragment,
		ForceQuery:  forceQuery,
		OmitHost:    omitHost,
	}

	fullURL := url.String()
	attrs = append(attrs, semconv.URLFull(fullURL))

	// Server address and port
	serverAddr, serverPort := http.ServerAddressPortAttributes(e.Host[:])
	if serverAddr.Valid() {
		attrs = append(attrs, serverAddr)
	}
	if serverPort.Valid() {
		attrs = append(attrs, serverPort)
	}

	proto := unix.ByteSliceToString(e.Proto[:])
	if proto != "" {
		parts := strings.Split(proto, "/")
		if len(parts) == 2 {
			if parts[0] != "HTTP" {
				attrs = append(attrs, semconv.NetworkProtocolName(parts[0]))
			}
			scheme = strings.ToLower(parts[0])
			attrs = append(attrs, semconv.NetworkProtocolVersion(parts[1]))
		}
	}

	spanEvent := &probe.SpanEvent{
		SpanName:          method,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		Attributes:        attrs,
		ParentSpanContext: pscPtr,
	}

	if int(e.StatusCode) >= 400 && int(e.StatusCode) < 600 {
		spanEvent.Status = probe.Status{Code: codes.Error}
	}

	return []*probe.SpanEvent{spanEvent}
}
