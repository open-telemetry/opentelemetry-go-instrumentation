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

package consumer

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
	spanID := trace.SpanID{1}

	got := convertEvent(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   uint64(start.UnixNano()),
			EndTime:     uint64(end.UnixNano()),
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

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &probe.SpanEvent{
		SpanName:    kafkaConsumerSpanName("topic1"),
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.MessagingSystemKafka,
			semconv.MessagingOperationTypeReceive,
			semconv.MessagingDestinationPartitionID("12"),
			semconv.MessagingDestinationName("topic1"),
			semconv.MessagingKafkaMessageOffset(42),
			semconv.MessagingKafkaMessageKey("key1"),
			semconv.MessagingKafkaConsumerGroup("test consumer group"),
		},
	}
	assert.Equal(t, want, got[0])
}
