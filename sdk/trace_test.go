// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	attrs = []attribute.KeyValue{
		attribute.Bool("bool", true),
		attribute.Int("int", -1),
		attribute.Int64("int64", 43),
		attribute.Float64("float64", 0.3),
		attribute.String("string", "value"),
		attribute.BoolSlice("bool slice", []bool{true, false, true}),
		attribute.IntSlice("int slice", []int{-1, -30, 328}),
		attribute.Int64Slice("int64 slice", []int64{1030, 0, 0}),
		attribute.Float64Slice("float64 slice", []float64{1e9}),
		attribute.StringSlice("string slice", []string{"one", "two"}),
	}

	spanContext0 = trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1},
		SpanID:     trace.SpanID{0x1},
		TraceFlags: trace.FlagsSampled,
	})
)

func TestSpanNilUnsampledGuards(t *testing.T) {
	run := func(fn func(s *span)) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()

			f := func(s *span) func() { return func() { fn(s) } }
			assert.NotPanics(t, f(nil), "nil span")
			assert.NotPanics(t, f(new(span)), "unsampled span")
		}
	}

	t.Run("End", run(func(s *span) { s.End() }))
	t.Run("AddEvent", run(func(s *span) { s.AddEvent("event name") }))
	t.Run("AddLink", run(func(s *span) { s.AddLink(trace.Link{}) }))
	t.Run("IsRecording", run(func(s *span) { _ = s.IsRecording() }))
	t.Run("RecordError", run(func(s *span) { s.RecordError(nil) }))
	t.Run("SpanContext", run(func(s *span) { _ = s.SpanContext() }))
	t.Run("SetStatus", run(func(s *span) { s.SetStatus(codes.Error, "test") }))
	t.Run("SetName", run(func(s *span) { s.SetName("span name") }))
	t.Run("SetAttributes", run(func(s *span) { s.SetAttributes(attrs...) }))
	t.Run("TracerProvider", run(func(s *span) { _ = s.TracerProvider() }))
}

func TestSpanIsRecording(t *testing.T) {
	builder := spanBuilder{}
	s := builder.Build()
	assert.True(t, s.IsRecording(), "sampled span should be recorded")

	builder.NotSampled = true
	s = builder.Build()
	assert.False(t, s.IsRecording(), "unsampled span should not be recorded")
}

func TestSpanSpanContext(t *testing.T) {
	s := spanBuilder{SpanContext: spanContext0}.Build()
	assert.Equal(t, spanContext0, s.SpanContext())
}

func TestSpanTracerProvider(t *testing.T) {
	var s span

	got := s.TracerProvider()
	assert.IsType(t, tracerProvider{}, got)
}

type spanBuilder struct {
	Name        string
	NotSampled  bool
	SpanContext trace.SpanContext
	Options     []trace.SpanStartOption
}

func (b spanBuilder) Build() *span {
	tracer := new(tracer)
	s := &span{sampled: !b.NotSampled, spanContext: b.SpanContext}
	s.traces, s.span = tracer.traces(
		context.Background(),
		b.Name,
		trace.NewSpanStartConfig(b.Options...),
		s.spanContext,
		trace.SpanContext{},
	)

	return s
}
