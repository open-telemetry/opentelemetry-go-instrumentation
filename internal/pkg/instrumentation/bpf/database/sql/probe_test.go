// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/kernel"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/pdataconv"
)

func BenchmarkProcessFn(b *testing.B) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "no query (baseline)",
			query: "",
		},
		{
			name:  "simple query",
			query: "SELECT * FROM customers",
		},
		{
			name:  "medium query",
			query: "SELECT * FROM customers WHERE first_name='Mike' AND last_name IN ('Santa', 'Banana')",
		},
		{
			name:  "hard query",
			query: "WITH (SELECT last_name FROM customers WHERE first_name='Mike' AND country='North Pole') AS test_table SELECT * FROM test_table WHERE first_name='Mike' AND last_name IN ('Santa', 'Banana')",
		},
	}

	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := kernel.TimeToBootOffset(start)
	endOffset := kernel.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	for _, t := range tests {
		b.Run(t.name, func(b *testing.B) {
			var byteQuery [256]byte
			copy(byteQuery[:], []byte(t.query))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = processFn(&event{
					BaseSpanProperties: context.BaseSpanProperties{
						StartTime:   startOffset,
						EndTime:     endOffset,
						SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
					},
					Query: byteQuery,
				})
			}
		})
	}
}

func TestProbeConvertEvent(t *testing.T) {
	t.Setenv(ParseDBStatementEnvVar, "true")
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := kernel.TimeToBootOffset(start)
	endOffset := kernel.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	got := processFn(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   startOffset,
			EndTime:     endOffset,
			SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
		},
		// "SELECT * FROM foo"
		Query: [256]byte{
			0x53, 0x45, 0x4c, 0x45, 0x43, 0x54, 0x20, 0x2a, 0x20,
			0x46, 0x52, 0x4f, 0x4d, 0x20, 0x66, 0x6f, 0x6f,
		},
	})

	want := func() ptrace.SpanSlice {
		spans := ptrace.NewSpanSlice()
		span := spans.AppendEmpty()
		span.SetName("SELECT foo")
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(kernel.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(kernel.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID(spanID))
		span.SetFlags(uint32(trace.FlagsSampled))
		pdataconv.Attributes(
			span.Attributes(),
			semconv.DBQueryText("SELECT * FROM foo"),
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("foo"),
		)
		return spans
	}()
	assert.Equal(t, want, got)
}
