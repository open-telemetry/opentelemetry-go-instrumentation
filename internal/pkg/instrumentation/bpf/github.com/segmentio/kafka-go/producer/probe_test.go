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

package producer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Second)

	traceID := trace.TraceID{1}

	got := convertEvent(&event{
		StartTime: uint64(start.UnixNano()),
		EndTime:   uint64(end.UnixNano()),
		TraceID:   traceID,
		Messages: [10]messageAttributes{
			{
				// topic1
				Topic: [256]byte{0x74, 0x6f, 0x70, 0x69, 0x63, 0x31},
				// key1
				Key:   [256]byte{0x6b, 0x65, 0x79, 0x31},
				SpaID: trace.SpanID{1},
			},
			{
				// topic2
				Topic: [256]byte{0x74, 0x6f, 0x70, 0x69, 0x63, 0x32},
				// key2
				Key:   [256]byte{0x6b, 0x65, 0x79, 0x32},
				SpaID: trace.SpanID{2},
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
	}
	assert.Equal(t, want1, got[0])
	assert.Equal(t, want2, got[1])
}
