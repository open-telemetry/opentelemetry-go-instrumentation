// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gin

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"

	"github.com/stretchr/testify/assert"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
)

func TestProbeConvertEvent(t *testing.T) {
	startTime := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	endTime := startTime.Add(1 * time.Second)

	startTimeOffset := utils.TimeToBootOffset(startTime)
	endTimeOffset := utils.TimeToBootOffset(endTime)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	testCases := []struct {
		name     string
		event    *event
		expected ptrace.SpanSlice
	}{
		{
			name: "basic client event",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				// "GET"
				Method: [8]byte{0x47, 0x45, 0x54},
				// "/foo/bar"
				Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
				// "/foo/bar"
				PathPattern: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("GET /foo/bar")
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startTimeOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endTimeOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/foo/bar"),
					semconv.HTTPRouteKey.String("/foo/bar"),
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
