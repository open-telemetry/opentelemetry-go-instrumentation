// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanNilUnsampledGuards(t *testing.T) {
	run := func(f func(s *span) func()) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()

			var s *span
			assert.NotPanics(t, f(s), "nil span")

			s = new(span)
			assert.NotPanics(t, f(s), "unsampled span")
		}
	}

	t.Run("End", run(func(s *span) func() {
		return func() { s.End() }
	}))

	t.Run("AddEvent", run(func(s *span) func() {
		return func() { s.AddEvent("event name") }
	}))

	t.Run("AddLink", run(func(s *span) func() {
		return func() { s.AddLink(trace.Link{}) }
	}))

	t.Run("IsRecording", run(func(s *span) func() {
		return func() { _ = s.IsRecording() }
	}))

	t.Run("RecordError", run(func(s *span) func() {
		return func() { s.RecordError(nil) }
	}))

	t.Run("SpanContext", run(func(s *span) func() {
		return func() { _ = s.SpanContext() }
	}))

	t.Run("SetStatus", run(func(s *span) func() {
		return func() { s.SetStatus(codes.Error, "test") }
	}))

	t.Run("SetName", run(func(s *span) func() {
		return func() { s.SetName("span name") }
	}))

	t.Run("SetAttributes", run(func(s *span) func() {
		return func() { s.SetAttributes(attribute.Bool("key", true)) }
	}))

	t.Run("TracerProvider", run(func(s *span) func() {
		return func() { _ = s.TracerProvider() }
	}))
}

func TestSpanSetName(t *testing.T) {
	const name = "span name"
	_, s := spanBuilder{Sampled: true}.Build(context.Background())
	s.SetName(name)
	assert.Equal(t, name, s.span.Name(), "sampled span name not set")
}

func TestSpanTracerProvider(t *testing.T) {
	var s span

	got := s.TracerProvider()
	require.IsType(t, &tracerProvider{}, got)
	assert.Same(t, got.(*tracerProvider), tracerProviderInstance)
}

type spanBuilder struct {
	Name              string
	Sampled           bool
	SpanContext       trace.SpanContext
	ParentSpanContext trace.SpanContext
	Options           []trace.SpanStartOption

	Tracer *tracer
}

func (b spanBuilder) Build(ctx context.Context) (context.Context, *span) {
	if b.Tracer == nil {
		b.Tracer = new(tracer)
	}

	s := &span{sampled: b.Sampled, spanContext: b.SpanContext}
	s.traces, s.span = b.Tracer.traces(
		ctx,
		b.Name,
		trace.NewSpanStartConfig(b.Options...),
		s.spanContext,
		b.ParentSpanContext,
	)

	ctx = trace.ContextWithSpan(ctx, s)
	return ctx, s
}
