// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redis

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
		// "set key value"
		Query: [256]byte{0x73, 0x65, 0x74, 0x20, 0x6B, 0x65, 0x79, 0x20, 0x76, 0x61, 0x6C, 0x75, 0x65},
	})

	want := func() ptrace.SpanSlice {
		spans := ptrace.NewSpanSlice()
		span := spans.AppendEmpty()
		span.SetName("DB")
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID(spanID))
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(span.Attributes(), semconv.DBQueryText("set key value"))
		return spans
	}()
	assert.Equal(t, want, got)
}
