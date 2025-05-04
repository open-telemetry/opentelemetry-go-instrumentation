// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package kafka provides an integration test for the Kafka probe.
package kafka

import (
	"encoding/hex"
	"regexp"
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

	spans := ptrace.NewSpanSlice()
	for i, scope := range scopes {
		t.Run("Scope/"+strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, scopeName, scope.Scope().Name(), "scope name")
			assert.Equal(t, semconv.SchemaURL, scope.SchemaUrl(), "schema URL")

			scope.Spans().MoveAndAppendTo(spans)
		})
	}

	// All trace ID should be the same.
	tIDBytes := [16]byte(spans.At(0).TraceID())
	tID := hex.EncodeToString(tIDBytes[:])

	var producerSpanIDs []string
	var cCnt int
	for j := range spans.Len() {
		b := [16]byte(spans.At(j).TraceID())
		assert.Equalf(t, tID, hex.EncodeToString(b[:]), "Span %d: trace ID", j)

		span := spans.At(j)
		switch span.Kind() {
		case ptrace.SpanKindProducer:
			n := len(producerSpanIDs)

			b := [8]byte(span.SpanID())
			sID := hex.EncodeToString(b[:])
			producerSpanIDs = append(producerSpanIDs, sID)

			t.Run("ProducerSpan/"+strconv.Itoa(n), pSpan(span))
		case ptrace.SpanKindConsumer:
			t.Run("ConsumerSpan/"+strconv.Itoa(cCnt), cSpan(span))
			cCnt++
		default:
			t.Errorf("unexpected span kind: %v", span.Kind())
		}
	}

	if cCnt == 0 {
		t.Error("no consumer span found")
		return
	}

	if len(producerSpanIDs) == 0 {
		t.Error("no producer span found")
		return
	}

	// Only the first consumer span is guarnteed to have a parent span ID from
	// our producers. This selcets the first consumer span.
	var cSpan *ptrace.Span
	for i := range spans.Len() {
		if spans.At(i).Kind() == ptrace.SpanKindConsumer {
			s := spans.At(i)
			cSpan = &s
			break
		}
	}
	if cSpan == nil {
		t.Fatal("no consumer span found")
	}
	b := [8]byte(cSpan.ParentSpanID())
	sID := hex.EncodeToString(b[:])
	assert.Contains(t, producerSpanIDs, sID)
}

var (
	producerNameRE = regexp.MustCompile(`^topic\d publish$`)
	topicRe        = regexp.MustCompile(`^topic\d$`)
	keyRe          = regexp.MustCompile(`^key\d$`)
)

func pSpan(span ptrace.Span) func(t *testing.T) {
	return func(t *testing.T) {
		e2e.AssertTraceID(t, span.TraceID(), "trace ID")
		e2e.AssertSpanID(t, span.SpanID(), "span ID")

		assert.Equal(t, ptrace.SpanKindProducer, span.Kind(), "span kind")

		assert.Regexp(t, producerNameRE, span.Name(), "span name")

		attrs := e2e.AttributesMap(span.Attributes())
		assert.Equal(
			t,
			"kafka",
			attrs[string(semconv.MessagingSystemKey)],
			"messaging.system",
		)
		assert.Regexp(
			t,
			topicRe,
			attrs[string(semconv.MessagingDestinationNameKey)],
			"messaging.destination.name",
		)
		assert.Regexp(
			t,
			keyRe,
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

func cSpan(span ptrace.Span) func(t *testing.T) {
	return func(t *testing.T) {
		e2e.AssertTraceID(t, span.TraceID(), "trace ID")
		e2e.AssertSpanID(t, span.SpanID(), "span ID")
		e2e.AssertSpanID(t, span.ParentSpanID(), "parent span ID")

		assert.Equal(t, ptrace.SpanKindConsumer, span.Kind(), "span kind")

		assert.Equal(t, "topic1 receive", span.Name(), "span name")

		attrs := e2e.AttributesMap(span.Attributes())
		assert.Equal(
			t,
			"kafka",
			attrs[string(semconv.MessagingSystemKey)],
			"messaging.system",
		)
		assert.Equal(
			t,
			"topic1",
			attrs[string(semconv.MessagingDestinationNameKey)],
			"messaging.destination.name",
		)
		assert.Equal(
			t,
			"key1",
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
