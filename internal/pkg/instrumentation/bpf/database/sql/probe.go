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

package sql

import (
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "database/sql"

	// IncludeDBStatementEnvVar is the environment variable to opt-in for sql query inclusion in the trace.
	IncludeDBStatementEnvVar = "OTEL_GO_AUTO_INCLUDE_DB_STATEMENT"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger.WithName(id.String()),
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
			StartTime:   int64(e.StartTime),
			EndTime:     int64(e.EndTime),
			SpanContext: &sc,
			Attributes: []attribute.KeyValue{
				semconv.DBQueryText(query),
			},
			ParentSpanContext: pscPtr,
		},
	}
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
