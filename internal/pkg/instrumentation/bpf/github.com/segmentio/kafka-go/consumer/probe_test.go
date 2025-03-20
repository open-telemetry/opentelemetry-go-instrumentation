// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
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
		// topic1
		Topic: [256]byte{0x74, 0x6f, 0x70, 0x69, 0x63, 0x31},
		// key1
		Key: [256]byte{0x6b, 0x65, 0x79, 0x31},
		// test consumer group
		ConsumerGroup: [128]byte{0x74, 0x65, 0x73, 0x74, 0x20, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x20, 0x67, 0x72, 0x6f, 0x75, 0x70},
		Offset:        42,
		Partition:     12,
	})

	want := func() ptrace.SpanSlice {
		spans := ptrace.NewSpanSlice()
		span := spans.AppendEmpty()
		span.SetName(kafkaConsumerSpanName("topic1"))
		span.SetKind(ptrace.SpanKindConsumer)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID(spanID))
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(
			span.Attributes(),
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypeReceive,
			semconv.MessagingDestinationPartitionID("12"),
			semconv.MessagingDestinationName("topic1"),
			semconv.MessagingKafkaOffset(42),
			semconv.MessagingKafkaMessageKey("key1"),
			semconv.MessagingConsumerGroupName("test consumer group"),
		)
		return spans
	}()
	assert.Equal(t, want, got)
}
