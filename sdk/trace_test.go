// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"errors"
	"math"
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

	spanContext0 = trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1},
		SpanID:     trace.SpanID{0x1},
		TraceFlags: trace.FlagsSampled,
	})
	spanContext1 = trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x2},
		SpanID:     trace.SpanID{0x2},
		TraceFlags: trace.FlagsSampled,
	})

	link0 = trace.Link{
		SpanContext: spanContext0,
		Attributes: []attribute.KeyValue{
			attribute.Int("n", 0),
		},
	}
	link1 = trace.Link{
		SpanContext: spanContext1,
		Attributes: []attribute.KeyValue{
			attribute.Int("n", 1),
		},
	}

	pLink0 = func() ptrace.SpanLink {
		l := ptrace.NewSpanLink()
		l.SetTraceID(pcommon.TraceID(spanContext0.TraceID()))
		l.SetSpanID(pcommon.SpanID(spanContext0.SpanID()))
		l.SetFlags(uint32(spanContext0.TraceFlags()))
		l.Attributes().PutInt("n", 0)
		return l
	}()
	pLink1 = func() ptrace.SpanLink {
		l := ptrace.NewSpanLink()
		l.SetTraceID(pcommon.TraceID(spanContext1.TraceID()))
		l.SetSpanID(pcommon.SpanID(spanContext1.SpanID()))
		l.SetFlags(uint32(spanContext1.TraceFlags()))
		l.Attributes().PutInt("n", 1)
		return l
	}()
)

func TestSpanCreation(t *testing.T) {
	const (
		spanName   = "span name"
		tracerName = "go.opentelemetry.io/otel/sdk/test"
		tracerVer  = "v0.1.0"
	)

	ts := time.Now()

	tracer := GetTracerProvider().Tracer(
		tracerName,
		trace.WithInstrumentationVersion(tracerVer),
		trace.WithSchemaURL(semconv.SchemaURL),
	)

	assertTracer := func(traces ptrace.Traces) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()

			rs := traces.ResourceSpans()
			require.Equal(t, 1, rs.Len())
			sss := rs.At(0).ScopeSpans()
			require.Equal(t, 1, sss.Len())
			ss := sss.At(0)
			assert.Equal(t, tracerName, ss.Scope().Name(), "tracer name")
			assert.Equal(t, tracerVer, ss.Scope().Version(), "tracer version")
			assert.Equal(t, semconv.SchemaURL, ss.SchemaUrl(), "tracer schema URL")
		}
	}

	testcases := []struct {
		TestName string
		SpanName string
		Options  []trace.SpanStartOption
		Setup    func(*testing.T)
		Eval     func(*testing.T, context.Context, *span)
	}{
		{
			TestName: "SampledByDefault",
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)

				assert.True(t, s.sampled, "not sampled by default.")
			},
		},
		{
			TestName: "ParentSpanContext",
			Setup: func(t *testing.T) {
				orig := start
				t.Cleanup(func() { start = orig })
				start = func(_ context.Context, _ *span, psc *trace.SpanContext, _ *bool, _ *trace.SpanContext) {
					*psc = spanContext0
				}
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)

				want := spanContext0.SpanID().String()
				got := s.span.ParentSpanID().String()
				assert.Equal(t, want, got)
			},
		},
		{
			TestName: "SpanContext",
			Setup: func(t *testing.T) {
				orig := start
				t.Cleanup(func() { start = orig })
				start = func(_ context.Context, _ *span, _ *trace.SpanContext, _ *bool, sc *trace.SpanContext) {
					*sc = spanContext0
				}
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)

				str := func(i interface{ String() string }) string {
					return i.String()
				}
				assert.Equal(t, str(spanContext0.TraceID()), str(s.span.TraceID()), "trace ID")
				assert.Equal(t, str(spanContext0.SpanID()), str(s.span.SpanID()), "span ID")
				assert.Equal(t, uint32(spanContext0.TraceFlags()), s.span.Flags(), "flags")
				assert.Equal(t, str(spanContext0.TraceState()), s.span.TraceState().AsRaw(), "tracestate")
			},
		},
		{
			TestName: "NotSampled",
			Setup: func(t *testing.T) {
				orig := start
				t.Cleanup(func() { start = orig })
				start = func(_ context.Context, _ *span, _ *trace.SpanContext, s *bool, _ *trace.SpanContext) {
					*s = false
				}
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assert.False(t, s.sampled, "sampled")
			},
		},
		{
			TestName: "WithName",
			SpanName: spanName,
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, spanName, s.span.Name())
			},
		},
		{
			TestName: "WithSpanKind",
			Options: []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindClient),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, ptrace.SpanKindClient, s.span.Kind())
			},
		},
		{
			TestName: "WithTimestamp",
			Options: []trace.SpanStartOption{
				trace.WithTimestamp(ts),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, pcommon.NewTimestampFromTime(ts), s.span.StartTimestamp())
			},
		},
		{
			TestName: "WithAttributes",
			Options: []trace.SpanStartOption{
				trace.WithAttributes(attrs...),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, pAttrs, s.span.Attributes())
			},
		},
		{
			TestName: "WithLinks",
			Options: []trace.SpanStartOption{
				trace.WithLinks(link0, link1),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				want := ptrace.NewSpanLinkSlice()
				pLink0.CopyTo(want.AppendEmpty())
				pLink1.CopyTo(want.AppendEmpty())
				assert.Equal(t, want, s.span.Links())
			},
		},
	}

	ctx := context.Background()
	for _, tc := range testcases {
		t.Run(tc.TestName, func(t *testing.T) {
			if tc.Setup != nil {
				tc.Setup(t)
			}

			c, sIface := tracer.Start(ctx, tc.SpanName, tc.Options...)
			require.IsType(t, &span{}, sIface)
			s := sIface.(*span)

			tc.Eval(t, c, s)
		})
	}
}

func TestSpanKindTransform(t *testing.T) {
	tests := map[trace.SpanKind]ptrace.SpanKind{
		trace.SpanKind(-1):          ptrace.SpanKindUnspecified,
		trace.SpanKindUnspecified:   ptrace.SpanKindUnspecified,
		trace.SpanKind(math.MaxInt): ptrace.SpanKindUnspecified,

		trace.SpanKindInternal: ptrace.SpanKindInternal,
		trace.SpanKindServer:   ptrace.SpanKindServer,
		trace.SpanKindClient:   ptrace.SpanKindClient,
		trace.SpanKindProducer: ptrace.SpanKindProducer,
		trace.SpanKindConsumer: ptrace.SpanKindConsumer,
	}

	for in, want := range tests {
		assert.Equal(t, want, spanKind(in), in.String())
	}
}

func TestSpanEnd(t *testing.T) {
	orig := ended
	t.Cleanup(func() { ended = orig })

	var buf []byte
	ended = func(b []byte) { buf = b }

	timeNow := time.Now()

	tests := []struct {
		Name    string
		Options []trace.SpanEndOption
		Eval    func(*testing.T, pcommon.Timestamp)
	}{
		{
			Name: "Now",
			Eval: func(t *testing.T, ts pcommon.Timestamp) {
				assert.False(t, ts.AsTime().IsZero(), "zero end time")
			},
		},
		{
			Name: "WithTimestamp",
			Options: []trace.SpanEndOption{
				trace.WithTimestamp(timeNow),
			},
			Eval: func(t *testing.T, ts pcommon.Timestamp) {
				assert.True(t, ts.AsTime().Equal(timeNow), "end time not set")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			s := spanBuilder{}.Build()
			s.End(test.Options...)

			assert.False(t, s.sampled, "ended span should not be sampled")
			require.NotNil(t, buf, "no span data emitted")

			var m ptrace.ProtoUnmarshaler
			traces, err := m.UnmarshalTraces(buf)
			require.NoError(t, err)

			rs := traces.ResourceSpans()
			require.Equal(t, 1, rs.Len())
			ss := rs.At(0).ScopeSpans()
			require.Equal(t, 1, ss.Len())
			spans := ss.At(0).Spans()
			require.Equal(t, 1, spans.Len())

			test.Eval(t, spans.At(0).EndTimestamp())
		})
	}
}

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

func TestSpanAddLink(t *testing.T) {
	s := spanBuilder{
		Options: []trace.SpanStartOption{trace.WithLinks(link0)},
	}.Build()
	s.AddLink(link1)

	want := ptrace.NewSpanLinkSlice()
	pLink0.CopyTo(want.AppendEmpty())
	pLink1.CopyTo(want.AppendEmpty())
	assert.Equal(t, want, s.span.Links())
}

func TestSpanIsRecording(t *testing.T) {
	builder := spanBuilder{}
	s := builder.Build()
	assert.True(t, s.IsRecording(), "sampled span should be recorded")

	builder.NotSampled = true
	s = builder.Build()
	assert.False(t, s.IsRecording(), "unsampled span should not be recorded")
}

func TestSpanRecordError(t *testing.T) {
	s := spanBuilder{}.Build()

	want := ptrace.NewSpanEventSlice()
	s.RecordError(nil)
	require.Equal(t, want, s.span.Events(), "nil error recorded")

	ts := time.Now()
	err := errors.New("test")
	s.RecordError(
		err,
		trace.WithTimestamp(ts),
		trace.WithAttributes(attribute.Bool("testing", true)),
	)
	e := want.AppendEmpty()
	e.SetName(semconv.ExceptionEventName)
	e.SetTimestamp(pcommon.NewTimestampFromTime(ts))
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

func TestSpanSpanContext(t *testing.T) {
	s := spanBuilder{SpanContext: spanContext0}.Build()
	assert.Equal(t, spanContext0, s.SpanContext())
}

func TestSpanSetStatus(t *testing.T) {
	s := spanBuilder{}.Build()

	want := ptrace.NewStatus()
	assert.Equal(t, want, s.span.Status(), "empty status should not be set")

	msg := "test"
	want.SetMessage(msg)

	for c, p := range map[codes.Code]ptrace.StatusCode{
		codes.Error: ptrace.StatusCodeError,
		codes.Ok:    ptrace.StatusCodeOk,
		codes.Unset: ptrace.StatusCodeUnset,
	} {
		want.SetCode(p)
		s.SetStatus(c, msg)
		assert.Equalf(t, want, s.span.Status(), "code: %s, msg: %s", c, msg)
	}
}

func TestSpanSetName(t *testing.T) {
	const name = "span name"
	builder := spanBuilder{}

	s := builder.Build()
	s.SetName(name)
	assert.Equal(t, name, s.span.Name(), "span name not set")

	builder.Name = "alt"
	s = builder.Build()
	s.SetName(name)
	assert.Equal(t, name, s.span.Name(), "SetName did not overwrite")
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
