// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rueidis

import (
	"fmt"
	"log/slog"
	"net"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	pkg = "github.com/redis/rueidis"
)

var v4InV6Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}

	uprobes := []*probe.Uprobe{
		{
			Sym:         "github.com/redis/rueidis.(*pipe).Do",
			EntryProbe:  "uprobe_pipe_Do",
			ReturnProbe: "uprobe_pipe_Do_Returns",
			PackageConstrainsts: []probe.PackageConstrainst{
				{
					Package:     "github.com/redis/rueidis",
					FailureMode: probe.FailureModeIgnore,
				},
			},
		},
	}

	// todo: does redis support distributed tracing? if yes, add propagation

	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.RegistersABIConst{},
				probe.AllocationConst{},
				// to extract peer address data
				probe.StructFieldConst{
					Key: "pipe_conn_pos",
					Val: structfield.NewID(pkg, pkg, "pipe", "conn"),
				},
				probe.StructFieldConst{
					Key: "tcp_conn_conn_pos",
					Val: structfield.NewID("std", "net", "TCPConn", "conn"),
				},
				probe.StructFieldConst{
					Key: "conn_fd_pos",
					Val: structfield.NewID("std", "net", "conn", "fd"),
				},
				probe.StructFieldConst{
					Key: "fd_raddr_pos",
					Val: structfield.NewID("std", "net", "netFD", "raddr"),
				},
				probe.StructFieldConst{
					Key: "tcp_addr_ip_pos",
					Val: structfield.NewID("std", "net", "TCPAddr", "IP"),
				},
				probe.StructFieldConst{
					Key: "tcp_addr_port_pos",
					Val: structfield.NewID("std", "net", "TCPAddr", "Port"),
				},
				// to extract command data
				probe.StructFieldConst{
					Key: "completed_cs_pos",
					Val: structfield.NewID(pkg, fmt.Sprintf("%s/%s", pkg, "internal/cmds"), "Completed", "cs"),
				},
				probe.StructFieldConst{
					Key: "cs_s_pos",
					Val: structfield.NewID(pkg, fmt.Sprintf("%s/%s", pkg, "internal/cmds"), "CommandSlice", "s"),
				},
				// to extract response data
				probe.StructFieldConst{
					Key: "result_error_pos",
					Val: structfield.NewID(pkg, pkg, "RedisResult", "err"),
				},
			},
			Uprobes: uprobes,
			SpecFn:  loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

// event represents a kafka message received by the consumer.
type event struct {
	context.BaseSpanProperties

	OperationName [20]byte
	LocalAddr     NetAddr
}

type NetAddr struct {
	IP   [16]uint8
	Port int32
}

func processFn(e *event) ptrace.SpanSlice {
	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	operation := unix.ByteSliceToString(e.OperationName[:])
	span.SetName(fmt.Sprintf("%s %s", "cache", operation))

	attrs := []attribute.KeyValue{}

	ip := parseU8SliceToIP(e.LocalAddr.IP)

	attrs = append(attrs, semconv.ServerAddress(ip.String()))
	attrs = append(attrs, semconv.DBOperationName(operation))
	attrs = append(attrs, semconv.DBSystemRedis)

	// document: https://opentelemetry.io/docs/specs/semconv/database/redis/
	// span.Attributes().PutStr("db.namespace", "redis")            // todo: hard to get as its set at the beginning of connection
	// span.Attributes().PutStr("db.collection.name", "redis")      // todo: maybe for zget, hget, mget and stuff?
	// span.Attributes().PutStr("db.response.status_code", "redis") // todo
	// span.Attributes().PutStr("error.type", "redis")              // todo: later
	// span.Attributes().PutStr("db.query.text", "redis")           // todo

	utils.Attributes(span.Attributes(), attrs...)

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	return spans
}

func parseU8SliceToIP(raw [16]uint8) net.IP {
	ip := make(net.IP, len(raw))
	copy(ip, raw[:])

	if isZeros(raw[4:16]) {
		copy(ip[12:16], raw[0:4])
		copy(ip[0:12], v4InV6Prefix)
	}
	return ip
}

func isZeros(p net.IP) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != 0 {
			return false
		}
	}
	return true
}
