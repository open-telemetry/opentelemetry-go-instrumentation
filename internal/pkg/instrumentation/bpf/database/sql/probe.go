// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/xwb1989/sqlparser"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "database/sql"

	// IncludeDBStatementEnvVar is the environment variable to opt-in for sql query inclusion in the trace.
	IncludeDBStatementEnvVar = "OTEL_GO_AUTO_INCLUDE_DB_STATEMENT"

	// ParseDBStatementEnvVar is the environment variable to opt-in for sql query operation in the trace.
	ParseDBStatementEnvVar = "OTEL_GO_AUTO_PARSE_DB_STATEMENT"
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
				probe.AllocationConst{},
				probe.KeyValConst{
					Key: "should_include_db_statement",
					Val: shouldIncludeDBStatement(),
				},
			},
			Uprobes: []*probe.Uprobe{
				{
					Sym:         "database/sql.(*DB).queryDC",
					EntryProbe:  "uprobe_queryDC",
					ReturnProbe: "uprobe_queryDC_Returns",
					FailureMode: probe.FailureModeIgnore,
				},
				{
					Sym:         "database/sql.(*DB).execDC",
					EntryProbe:  "uprobe_execDC",
					ReturnProbe: "uprobe_execDC_Returns",
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
	Query [256]byte
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

	query := unix.ByteSliceToString(e.Query[:])
	if query != "" {
		span.Attributes().PutStr(string(semconv.DBQueryTextKey), query)
	}

	includeOperationVal := os.Getenv(ParseDBStatementEnvVar)
	if includeOperationVal != "" {
		include, err := strconv.ParseBool(includeOperationVal)
		if err == nil && include {
			operation, target, err := Parse(query)
			if err == nil {
				name := ""
				if operation != "" {
					span.Attributes().PutStr(string(semconv.DBOperationNameKey), operation)
					name = operation
				}
				if target != "" {
					span.Attributes().PutStr(string(semconv.DBCollectionNameKey), target)
					if name != "" {
						// if operation is in the name and target is available, set name to {operation} {target}
						name += " " + target
					}
				}
				if name != "" {
					span.SetName(name)
				}
			}
		}
	}

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

// Parse takes a SQL query string and returns the parsed query statement type
// and table name, or an error if parsing failed.
func Parse(query string) (string, string, error) {
	stmt, err := sqlparser.Parse(query)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse query: %w", err)
	}

	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		return "SELECT", getTableName(stmt.From), nil
	case *sqlparser.Update:
		return "UPDATE", getTableName(stmt.TableExprs), nil
	case *sqlparser.Insert:
		return "INSERT", stmt.Table.Name.String(), nil
	case *sqlparser.Delete:
		return "DELETE", getTableName(stmt.TableExprs), nil
	default:
		return "", "", errors.New("unsupported operation")
	}
}

// getTableName extracts the table name from a SQL node.
func getTableName(node sqlparser.SQLNode) string {
	switch tableExpr := node.(type) {
	case sqlparser.TableName:
		return tableExpr.Name.String()
	case sqlparser.TableExprs:
		for _, expr := range tableExpr {
			if tableName, ok := expr.(*sqlparser.AliasedTableExpr); ok {
				if name, ok := tableName.Expr.(sqlparser.TableName); ok {
					return name.Name.String()
				}
			}
		}
	}
	return ""
}
