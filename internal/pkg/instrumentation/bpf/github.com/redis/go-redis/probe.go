// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "github.com/go-redis/redis/v8"

	// IncludeDBStatementEnvVar is the environment variable to opt-in for sql query inclusion in the trace.
	IncludeDBStatementEnvVar = "OTEL_GO_AUTO_INCLUDE_DB_STATEMENT"

	// TODO:
	// ParseDBStatementEnvVar is the environment variable to opt-in for sql query operation in the trace.
	// ParseDBStatementEnvVar = "OTEL_GO_AUTO_PARSE_DB_STATEMENT".
)

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
				probe.RegistersABIConst{},
				probe.AllocationConst{},
				probe.KeyValConst{
					Key: "should_include_db_statement",
					Val: shouldIncludeDBStatement(),
				},
			},
			Uprobes: []probe.Uprobe{
				{
					Sym:         "github.com/go-redis/redis/v8/internal/pool.(*Conn).WithWriter",
					EntryProbe:  "uprobe_WithWriter",
					ReturnProbe: "uprobe_WithWriter_Returns",
					FailureMode: probe.FailureModeIgnore,
				},
				// `go-redis/v9` requires comprehensive testing.
				{
					Sym:         "github.com/redis/go-redis/v9/internal/pool.(*Conn).WithWriter",
					EntryProbe:  "uprobe_WithWriter",
					ReturnProbe: "uprobe_WithWriter_Returns",
					FailureMode: probe.FailureModeIgnore,
				},

				// Pipelining mode
				{
					Sym:         "github.com/go-redis/redis/v8.(*baseClient).generalProcessPipeline",
					EntryProbe:  "uprobe_processPipeline",
					ReturnProbe: "uprobe_processPipeline_Returns",
					FailureMode: probe.FailureModeIgnore,
				},
				{
					Sym:         "github.com/redis/go-redis/v9.(*baseClient).generalProcessPipeline",
					EntryProbe:  "uprobe_processPipeline",
					ReturnProbe: "uprobe_processPipeline_Returns",
					FailureMode: probe.FailureModeIgnore,
				},
			},

			SpecFn: loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

// event represents an event in an SQL database
// request-response.
type event struct {
	context.BaseSpanProperties
	RespMsg [256]byte
	Segs    int32 // Only be set under pipelining mode
}

func processFn(e *event) ptrace.SpanSlice {
	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName("DB")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}
	shouldIncludeStmt := shouldIncludeDBStatement()
	stmts, err := utils.ParseRESP(e.Segs, e.RespMsg[:])
	var query string
	if err != nil && shouldIncludeStmt {
		query = "[DB STATEMENT NOT SUPPORTED]"
	} else if shouldIncludeStmt {
		query = strings.Join(stmts, ", ")
	}
	if query != "" {
		span.Attributes().PutStr(string(semconv.DBQueryTextKey), query)
	}

	// TODO:
	// Add attr semconv.DBOperationNameKey(db.operation.name) & semconv.DBCollectionNameKey(db.collection.name)
	// which means to complete the logic like `ParseDBStatementEnvVar` in database/sql

	return spans
}

// shouldIncludeDBStatement returns if the user has configured SQL queries to be included.
func shouldIncludeDBStatement() bool {
	val := os.Getenv(IncludeDBStatementEnvVar)
	if val != "" {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}
