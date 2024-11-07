// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := utils.TimeToBootOffset(start)
	endOffset := utils.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	got := processFn(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   startOffset,
			EndTime:     endOffset,
			SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
		},
		// "SELECT * FROM foo"
		Query: [256]byte{0x53, 0x45, 0x4c, 0x45, 0x43, 0x54, 0x20, 0x2a, 0x20, 0x46, 0x52, 0x4f, 0x4d, 0x20, 0x66, 0x6f, 0x6f},
	})

	want := func() ptrace.SpanSlice {
		spans := ptrace.NewSpanSlice()
		span := spans.AppendEmpty()
		span.SetName("SELECT")
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID(spanID))
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(span.Attributes(), semconv.DBQueryText("SELECT * FROM foo"), semconv.DBOperationName("SELECT"))
		return spans
	}()
	assert.Equal(t, want, got)
}
