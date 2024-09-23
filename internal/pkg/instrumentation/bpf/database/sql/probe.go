// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "database/sql"

	// IncludeDBStatementEnvVar is the environment variable to opt-in for sql query inclusion in the trace.
	IncludeDBStatementEnvVar = "OTEL_GO_AUTO_INCLUDE_DB_STATEMENT"
)

type Config struct {
	// IncludeQueryText enables or disables inclusion of sql query text in the trace.
	IncludeQueryText bool
}

func (c *Config) Package() string {
	return pkg
}

// New returns a new [probe.Probe].
func New(logger *slog.Logger, config probe.Config) probe.Probe {
	cfg := config.(*Config)

	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:          id,
		ProbeConfig: cfg,
		Logger:      logger,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.KeyValConst{
				Key: "should_include_db_statement",
				Val: cfg.IncludeQueryText,
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:         "database/sql.(*DB).queryDC",
				EntryProbe:  "uprobe_queryDC",
				ReturnProbe: "uprobe_queryDC_Returns",
				Optional:    true,
			},
			{
				Sym:         "database/sql.(*DB).execDC",
				EntryProbe:  "uprobe_execDC",
				ReturnProbe: "uprobe_execDC_Returns",
				Optional:    true,
			},
		},

		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

// event represents an event in an SQL database
// request-response.
type event struct {
	context.BaseSpanProperties
	Query [256]byte
}

func convertEvent(e *event) []*probe.SpanEvent {
	query := unix.ByteSliceToString(e.Query[:])

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

	return []*probe.SpanEvent{
		{
			SpanName:    "DB",
			StartTime:   utils.BootOffsetToTime(e.StartTime),
			EndTime:     utils.BootOffsetToTime(e.EndTime),
			SpanContext: &sc,
			Attributes: []attribute.KeyValue{
				semconv.DBQueryText(query),
			},
			ParentSpanContext: pscPtr,
			TracerSchema:      semconv.SchemaURL,
		},
	}
}
