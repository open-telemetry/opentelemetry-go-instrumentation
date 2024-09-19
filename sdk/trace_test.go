// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

	pAttrs = func() pcommon.Map {
		m := pcommon.NewMap()
		m.PutBool("bool", true)
		m.PutInt("int", -1)
		m.PutInt("int64", 43)
		m.PutDouble("float64", 0.3)
		m.PutStr("string", "value")

		s := m.PutEmptySlice("bool slice")
		s.AppendEmpty().SetBool(true)
		s.AppendEmpty().SetBool(false)
		s.AppendEmpty().SetBool(true)

		s = m.PutEmptySlice("int slice")
		s.AppendEmpty().SetInt(-1)
		s.AppendEmpty().SetInt(-30)
		s.AppendEmpty().SetInt(328)

		s = m.PutEmptySlice("int64 slice")
		s.AppendEmpty().SetInt(1030)
		s.AppendEmpty().SetInt(0)
		s.AppendEmpty().SetInt(0)

		s = m.PutEmptySlice("float64 slice")
		s.AppendEmpty().SetDouble(1e9)

		s = m.PutEmptySlice("string slice")
		s.AppendEmpty().SetStr("one")
		s.AppendEmpty().SetStr("two")

		return m
	}()
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

func TestSpanSetAttributes(t *testing.T) {
	builder := spanBuilder{}

	s := builder.Build()
	s.SetAttributes(attrs...)
	assert.Equal(t, pAttrs, s.span.Attributes(), "span attributes not set")

	builder.Options = []trace.SpanStartOption{
		trace.WithAttributes(attrs[0].Key.Bool(!attrs[0].Value.AsBool())),
	}

	s = builder.Build()
	s.SetAttributes(attrs...)
	assert.Equal(t, pAttrs, s.span.Attributes(), "SpanAttributes did not override")
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
