// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer

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

	ver := "v1"
	got := processFn(pkg, ver, semconv.SchemaURL)(&event{
		StartTime: startOffset,
		EndTime:   endOffset,
		Messages: [10]messageAttributes{
			{
				// topic1
				Topic: [256]byte{0x74, 0x6f, 0x70, 0x69, 0x63, 0x31},
				// key1
				Key: [256]byte{0x6b, 0x65, 0x79, 0x31},
				SpanContext: context.EBPFSpanContext{
					TraceID: traceID,
					SpanID:  trace.SpanID{1},
				},
			},
			{
				// topic2
				Topic: [256]byte{0x74, 0x6f, 0x70, 0x69, 0x63, 0x32},
				// key2
				Key: [256]byte{0x6b, 0x65, 0x79, 0x32},
				SpanContext: context.EBPFSpanContext{
					TraceID: traceID,
					SpanID:  trace.SpanID{2},
				},
			},
		},
		ValidMessages: 2,
	})

	want := func() ptrace.ScopeSpans {
		ss := ptrace.NewScopeSpans()

		ss.Scope().SetName("go.opentelemetry.io/auto/" + pkg)
		ss.Scope().SetVersion(ver)
		ss.SetSchemaUrl(semconv.SchemaURL)

		span := ss.Spans().AppendEmpty()
		span.SetName(kafkaProducerSpanName("topic1"))
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID{1})
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(
			span.Attributes(),
			semconv.MessagingKafkaMessageKey("key1"),
			semconv.MessagingDestinationName("topic1"),
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypePublish,
			semconv.MessagingBatchMessageCount(2),
		)

		span = ss.Spans().AppendEmpty()
		span.SetName(kafkaProducerSpanName("topic2"))
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID{2})
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(
			span.Attributes(),
			semconv.MessagingKafkaMessageKey("key2"),
			semconv.MessagingDestinationName("topic2"),
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypePublish,
			semconv.MessagingBatchMessageCount(2),
		)

		return ss
	}()
	assert.Equal(t, want, got)
}
