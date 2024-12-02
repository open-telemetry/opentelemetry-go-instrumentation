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

package chi

import (
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
	"time"

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
				span.SetName("GET")
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
