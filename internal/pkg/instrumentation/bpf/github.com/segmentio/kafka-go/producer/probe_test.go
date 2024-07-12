// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Second)

	traceID := trace.TraceID{1}

	got := convertEvent(&event{
		StartTime: uint64(start.UnixNano()),
		EndTime:   uint64(end.UnixNano()),
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

	sc1 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
	})
	sc2 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	want1 := &probe.SpanEvent{
		SpanName:    kafkaProducerSpanName("topic1"),
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc1,
		Attributes: []attribute.KeyValue{
			semconv.MessagingKafkaMessageKey("key1"),
			semconv.MessagingDestinationName("topic1"),
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypePublish,
			semconv.MessagingBatchMessageCount(2),
		},
		TracerSchema: semconv.SchemaURL,
	}

	want2 := &probe.SpanEvent{
		SpanName:    kafkaProducerSpanName("topic2"),
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc2,
		Attributes: []attribute.KeyValue{
			semconv.MessagingKafkaMessageKey("key2"),
			semconv.MessagingDestinationName("topic2"),
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypePublish,
			semconv.MessagingBatchMessageCount(2),
		},
		TracerSchema: semconv.SchemaURL,
	}
	assert.Equal(t, want1, got[0])
	assert.Equal(t, want2, got[1])
}
