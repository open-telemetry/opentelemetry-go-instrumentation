// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package kafka provides an integration test for the Kafka probe.
package kafka

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "go.opentelemetry.io/auto/github.com/segmentio/kafka-go"

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running integration test in short mode.")
	}

	defer goleak.VerifyNone(t)

	traces := e2e.RunInstrumentation(t, "./cmd")
	scopes := e2e.ScopeSpansByName(traces, scopeName)
	require.NotEmpty(t, scopes)

	t.Run("ResourceAttribute/ServiceName", func(t *testing.T) {
		val, err := e2e.ResourceAttribute(traces, "service.name")
		require.NoError(t, err)
		assert.Equal(t, "sample-app", val.AsString())
	})

	// All trace ID should be the same.
	tIDBytes := [16]byte(scopes[0].Spans().At(0).TraceID())
	tID := hex.EncodeToString(tIDBytes[:])

	var producerSpanIDs []string
	s, err := e2e.SelectSpan(scopes, func(span ptrace.Span) bool {
		return span.Kind() == ptrace.SpanKindProducer && span.Name() == "topic1 publish"
	})
	require.NoError(t, err, "producer span 'topic1 publish' not found")

	b := [8]byte(s.SpanID())
	sID := hex.EncodeToString(b[:])
	producerSpanIDs = append(producerSpanIDs, sID)
	t.Run("ProducerSpan/topic1", pSpan(1, tID, s))

	s, err = e2e.SelectSpan(scopes, func(span ptrace.Span) bool {
		return span.Kind() == ptrace.SpanKindProducer && span.Name() == "topic2 publish"
	})
	require.NoError(t, err, "producer span 'topic2 publish' not found")

	b = [8]byte(s.SpanID())
	sID = hex.EncodeToString(b[:])
	producerSpanIDs = append(producerSpanIDs, sID)
	t.Run("ProducerSpan/topic2", pSpan(2, tID, s))

	s, err = e2e.SelectSpan(scopes, func(span ptrace.Span) bool {
		return span.Kind() == ptrace.SpanKindConsumer && span.Name() == "topic1 receive"
	})
	require.NoError(t, err, "consumer span 'topic1 receive' not found")
	t.Run("ConsumerSpan/topic1", cSpan(1, tID, s))
	b = [8]byte(s.ParentSpanID())
	sID = hex.EncodeToString(b[:])
	assert.Contains(t, producerSpanIDs, sID)

	s, err = e2e.SelectSpan(scopes, func(span ptrace.Span) bool {
		return span.Kind() == ptrace.SpanKindConsumer && span.Name() == "topic2 receive"
	})
	require.NoError(t, err, "consumer span 'topic2 receive' not found")
	t.Run("ConsumerSpan/topic2", cSpan(2, tID, s))
	b = [8]byte(s.ParentSpanID())
	sID = hex.EncodeToString(b[:])
	assert.Contains(t, producerSpanIDs, sID)
}

func pSpan(n int, tID string, span ptrace.Span) func(t *testing.T) {
	return func(t *testing.T) {
		b := [16]byte(span.TraceID())
		assert.Equalf(t, tID, hex.EncodeToString(b[:]), "trace ID")
		e2e.AssertTraceID(t, span.TraceID(), "trace ID")
		e2e.AssertSpanID(t, span.SpanID(), "span ID")

		attrs := e2e.AttributesMap(span.Attributes())
		assert.Equal(
			t,
			"kafka",
			attrs[string(semconv.MessagingSystemKey)],
			"messaging.system",
		)
		assert.Equal(
			t,
			"topic"+strconv.Itoa(n),
			attrs[string(semconv.MessagingDestinationNameKey)],
			"messaging.destination.name",
		)
		assert.Equal(
			t,
			"key"+strconv.Itoa(n),
			attrs[string(semconv.MessagingKafkaMessageKeyKey)],
			"messaging.kafka.message.key",
		)
		assert.Equal(
			t,
			int64(2),
			attrs[string(semconv.MessagingBatchMessageCountKey)],
			"messaging.batch.message.count",
		)
	}
}

func cSpan(n int, tID string, span ptrace.Span) func(t *testing.T) {
	return func(t *testing.T) {
		b := [16]byte(span.TraceID())
		assert.Equalf(t, tID, hex.EncodeToString(b[:]), "trace ID")
		e2e.AssertSpanID(t, span.SpanID(), "span ID")
		e2e.AssertSpanID(t, span.ParentSpanID(), "parent span ID")

		attrs := e2e.AttributesMap(span.Attributes())
		assert.Equal(
			t,
			"kafka",
			attrs[string(semconv.MessagingSystemKey)],
			"messaging.system",
		)
		assert.Equal(
			t,
			"topic"+strconv.Itoa(n),
			attrs[string(semconv.MessagingDestinationNameKey)],
			"messaging.destination.name",
		)
		assert.Equal(
			t,
			"key"+strconv.Itoa(n),
			attrs[string(semconv.MessagingKafkaMessageKeyKey)],
			"messaging.kafka.message.key",
		)
		assert.Equal(
			t,
			"0",
			attrs[string(semconv.MessagingDestinationPartitionIDKey)],
			"messaging.destination.partition.id",
		)
		assert.Equal(
			t,
			"some group id",
			attrs[string(semconv.MessagingConsumerGroupNameKey)],
			"messaging.consumer.group.name",
		)
	}
}
