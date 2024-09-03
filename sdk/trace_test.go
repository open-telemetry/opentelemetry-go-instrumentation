// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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

func TestSpanRecordError(t *testing.T) {
	_, s := spanBuilder{}.Build(context.Background())

	want := ptrace.NewSpanEventSlice()
	s.RecordError(nil)
	require.Equal(t, want, s.span.Events(), "nil error recorded")

	now := time.Now()
	err := errors.New("test")
	s.RecordError(
		err,
		trace.WithTimestamp(now),
		trace.WithAttributes(attribute.Bool("testing", true)),
	)
	e := want.AppendEmpty()
	e.SetName(semconv.ExceptionEventName)
	e.SetTimestamp(pcommon.NewTimestampFromTime(now))
	e.Attributes().PutBool("testing", true)
	e.Attributes().PutStr(string(semconv.ExceptionTypeKey), "*errors.errorString")
	e.Attributes().PutStr(string(semconv.ExceptionMessageKey), err.Error())
	assert.Equal(t, want, s.span.Events(), "nil error recorded")

	s.RecordError(err, trace.WithStackTrace(true))
	require.Equal(t, 2, s.span.Events().Len(), "missing event")
	e = s.span.Events().At(1)
	_, ok := e.Attributes().Get(string(semconv.ExceptionStacktraceKey))
	assert.True(t, ok, "missing stacktrace attribute")
}

func TestSpanSetStatus(t *testing.T) {
	_, s := spanBuilder{}.Build(context.Background())

	want := ptrace.NewStatus()
	assert.Equal(t, want, s.span.Status())

	c, msg := codes.Error, "test"
	want.SetMessage(msg)
	want.SetCode(ptrace.StatusCodeError)
	s.SetStatus(c, msg)
	assert.Equalf(t, want, s.span.Status(), "code: %s, msg: %s", c, msg)

	c = codes.Ok
	want.SetCode(ptrace.StatusCodeOk)
	s.SetStatus(c, msg)
	assert.Equalf(t, want, s.span.Status(), "code: %s, msg: %s", c, msg)

	c = codes.Unset
	want.SetCode(ptrace.StatusCodeUnset)
	s.SetStatus(c, msg)
	assert.Equalf(t, want, s.span.Status(), "code: %s, msg: %s", c, msg)
}

func TestSpanSetName(t *testing.T) {
	const name = "span name"
	builder := spanBuilder{}

	_, s := builder.Build(context.Background())
	s.SetName(name)
	assert.Equal(t, name, s.span.Name(), "span name not set")

	builder.Name = "alt"
	_, s = builder.Build(context.Background())
	s.SetName(name)
	assert.Equal(t, name, s.span.Name(), "SetName overrides default")
}

func TestSpanSetAttributes(t *testing.T) {
	builder := spanBuilder{}

	_, s := builder.Build(context.Background())
	s.SetAttributes(attrs...)
	assert.Equal(t, pAttrs, s.span.Attributes(), "span attributes not set")

	builder.Options = []trace.SpanStartOption{
		trace.WithAttributes(attrs[0].Key.Bool(!attrs[0].Value.AsBool())),
	}

	_, s = builder.Build(context.Background())
	s.SetAttributes(attrs...)
	assert.Equal(t, pAttrs, s.span.Attributes(), "SpanAttributes did not override")
}

func TestSpanTracerProvider(t *testing.T) {
	var s span

	got := s.TracerProvider()
	require.IsType(t, &tracerProvider{}, got)
	assert.Same(t, got.(*tracerProvider), tracerProviderInstance)
}

type spanBuilder struct {
	Name              string
	NotSampled        bool
	SpanContext       trace.SpanContext
	ParentSpanContext trace.SpanContext
	Options           []trace.SpanStartOption

	Tracer *tracer
}

func (b spanBuilder) Build(ctx context.Context) (context.Context, *span) {
	if b.Tracer == nil {
		b.Tracer = new(tracer)
	}

	s := &span{sampled: !b.NotSampled, spanContext: b.SpanContext}
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
