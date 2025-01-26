// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rueidis

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"

	"github.com/stretchr/testify/assert"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := utils.TimeToBootOffset(start)
	endOffset := utils.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	testCases := []struct {
		name     string
		event    *event
		expected ptrace.SpanSlice
	}{
		{
			name: "basic get test",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startOffset,
					EndTime:     endOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				OperationName: [20]byte{0x47, 0x45, 0x54},
				LocalAddr: NetAddr{
					IP:   [16]uint8{172, 20, 0, 3},
					Port: 6379,
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("cache GET")
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				utils.Attributes(
					span.Attributes(),
					semconv.ServerAddress("172.20.0.3"),
					semconv.DBOperationName("GET"),
					semconv.DBSystemRedis,
				)

				return spans
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			out := processFn(tt.event)
			assert.Equal(t, tt.expected, out)
		})
	}
}
