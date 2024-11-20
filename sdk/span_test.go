// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/sdk/internal/telemetry"
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

	tAttrs = []telemetry.Attr{
		telemetry.Bool("bool", true),
		telemetry.Int("int", -1),
		telemetry.Int64("int64", 43),
		telemetry.Float64("float64", 0.3),
		telemetry.String("string", "value"),
		telemetry.Slice(
			"bool slice",
			telemetry.BoolValue(true),
			telemetry.BoolValue(false),
			telemetry.BoolValue(true),
		),
		telemetry.Slice("int slice",
			telemetry.IntValue(-1),
			telemetry.IntValue(-30),
			telemetry.IntValue(328),
		),
		telemetry.Slice("int64 slice",
			telemetry.Int64Value(1030),
			telemetry.Int64Value(0),
			telemetry.Int64Value(0),
		),
		telemetry.Slice("float64 slice", telemetry.Float64Value(1e9)),
		telemetry.Slice("string slice",
			telemetry.StringValue("one"),
			telemetry.StringValue("two"),
		),
	}

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

	tLink0 = &telemetry.SpanLink{
		TraceID: telemetry.TraceID(spanContext0.TraceID()),
		SpanID:  telemetry.SpanID(spanContext0.SpanID()),
		Flags:   uint32(spanContext0.TraceFlags()),
		Attrs:   []telemetry.Attr{telemetry.Int("n", 0)},
	}
	tLink1 = &telemetry.SpanLink{
		TraceID: telemetry.TraceID(spanContext1.TraceID()),
		SpanID:  telemetry.SpanID(spanContext1.SpanID()),
		Flags:   uint32(spanContext1.TraceFlags()),
		Attrs:   []telemetry.Attr{telemetry.Int("n", 1)},
	}
)

func TestSpanCreation(t *testing.T) {
	const (
		spanName   = "span name"
		tracerName = "go.opentelemetry.io/otel/sdk/test"
		tracerVer  = "v0.1.0"
	)

	ts := time.Now()

	tracer := TracerProvider().Tracer(
		tracerName,
		trace.WithInstrumentationVersion(tracerVer),
		trace.WithSchemaURL(semconv.SchemaURL),
	)

	assertTracer := func(traces *telemetry.Traces) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()

			rs := traces.ResourceSpans
			require.Len(t, rs, 1)
			sss := rs[0].ScopeSpans
			require.Len(t, sss, 1)
			ss := sss[0]
			assert.Equal(t, tracerName, ss.Scope.Name, "tracer name")
			assert.Equal(t, tracerVer, ss.Scope.Version, "tracer version")
			assert.Equal(t, semconv.SchemaURL, ss.SchemaURL, "tracer schema URL")
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

				assert.True(t, s.sampled.Load(), "not sampled by default.")
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
				got := s.span.ParentSpanID.String()
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
				assert.Equal(t, str(spanContext0.TraceID()), s.span.TraceID.String(), "trace ID")
				assert.Equal(t, str(spanContext0.SpanID()), s.span.SpanID.String(), "span ID")
				assert.Equal(t, uint32(spanContext0.TraceFlags()), s.span.Flags, "flags")
				assert.Equal(t, str(spanContext0.TraceState()), s.span.TraceState, "tracestate")
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
				assert.False(t, s.sampled.Load(), "sampled")
			},
		},
		{
			TestName: "WithName",
			SpanName: spanName,
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, spanName, s.span.Name)
			},
		},
		{
			TestName: "WithSpanKind",
			Options: []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindClient),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, telemetry.SpanKindClient, s.span.Kind)
			},
		},
		{
			TestName: "WithTimestamp",
			Options: []trace.SpanStartOption{
				trace.WithTimestamp(ts),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, ts, s.span.StartTime)
			},
		},
		{
			TestName: "WithAttributes",
			Options: []trace.SpanStartOption{
				trace.WithAttributes(attrs...),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				assert.Equal(t, tAttrs, s.span.Attrs)
			},
		},
		{
			TestName: "WithLinks",
			Options: []trace.SpanStartOption{
				trace.WithLinks(link0, link1),
			},
			Eval: func(t *testing.T, _ context.Context, s *span) {
				assertTracer(s.traces)
				want := []*telemetry.SpanLink{tLink0, tLink1}
				assert.Equal(t, want, s.span.Links)
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

func TestSpanEnd(t *testing.T) {
	orig := ended
	t.Cleanup(func() { ended = orig })

	var buf []byte
	ended = func(b []byte) { buf = b }

	timeNow := time.Unix(0, time.Now().UnixNano()) // No location.

	tests := []struct {
		Name    string
		Options []trace.SpanEndOption
		Eval    func(*testing.T, time.Time)
	}{
		{
			Name: "Now",
			Eval: func(t *testing.T, ts time.Time) {
				assert.False(t, ts.IsZero(), "zero end time")
			},
		},
		{
			Name: "WithTimestamp",
			Options: []trace.SpanEndOption{
				trace.WithTimestamp(timeNow),
			},
			Eval: func(t *testing.T, ts time.Time) {
				assert.Equal(t, timeNow, ts, "end time not set")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			s := spanBuilder{}.Build()
			s.End(test.Options...)

			assert.False(t, s.sampled.Load(), "ended span should not be sampled")
			require.NotNil(t, buf, "no span data emitted")

			var traces telemetry.Traces
			err := json.Unmarshal(buf, &traces)
			require.NoError(t, err)

			rs := traces.ResourceSpans
			require.Len(t, rs, 1)
			ss := rs[0].ScopeSpans
			require.Len(t, ss, 1)
			spans := ss[0].Spans
			require.Len(t, spans, 1)

			test.Eval(t, spans[0].EndTime)
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

	want := []*telemetry.SpanLink{tLink0, tLink1}
	assert.Equal(t, want, s.span.Links)
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

	var want []*telemetry.SpanEvent
	s.RecordError(nil)
	require.Equal(t, want, s.span.Events, "nil error recorded")

	ts := time.Now()
	err := errors.New("test")
	s.RecordError(
		err,
		trace.WithTimestamp(ts),
		trace.WithAttributes(attribute.Bool("testing", true)),
	)
	want = append(want, &telemetry.SpanEvent{
		Name: semconv.ExceptionEventName,
		Time: ts,
		Attrs: []telemetry.Attr{
			telemetry.Bool("testing", true),
			telemetry.String(string(semconv.ExceptionTypeKey), "*errors.errorString"),
			telemetry.String(string(semconv.ExceptionMessageKey), err.Error()),
		},
	})
	assert.Equal(t, want, s.span.Events, "nil error recorded")

	s.RecordError(err, trace.WithStackTrace(true))
	require.Len(t, s.span.Events, 2, "missing event")

	var hasST bool
	for _, attr := range s.span.Events[1].Attrs {
		if attr.Key == string(semconv.ExceptionStacktraceKey) {
			hasST = true
			break
		}
	}
	assert.True(t, hasST, "missing stacktrace attribute")
}

func TestSpanSpanContext(t *testing.T) {
	s := spanBuilder{SpanContext: spanContext0}.Build()
	assert.Equal(t, spanContext0, s.SpanContext())
}

func TestSpanSetStatus(t *testing.T) {
	s := spanBuilder{}.Build()

	assert.Nil(t, s.span.Status, "empty status should not be set")

	const msg = "test"
	want := &telemetry.Status{Message: msg}

	for c, tCode := range map[codes.Code]telemetry.StatusCode{
		codes.Error: telemetry.StatusCodeError,
		codes.Ok:    telemetry.StatusCodeOK,
		codes.Unset: telemetry.StatusCodeUnset,
	} {
		want.Code = tCode
		s.SetStatus(c, msg)
		assert.Equalf(t, want, s.span.Status, "code: %s, msg: %s", c, msg)
	}
}

func TestSpanSetName(t *testing.T) {
	const name = "span name"
	builder := spanBuilder{}

	s := builder.Build()
	s.SetName(name)
	assert.Equal(t, name, s.span.Name, "span name not set")

	builder.Name = "alt"
	s = builder.Build()
	s.SetName(name)
	assert.Equal(t, name, s.span.Name, "SetName did not overwrite")
}

func TestSpanSetAttributes(t *testing.T) {
	builder := spanBuilder{}

	s := builder.Build()
	s.SetAttributes(attrs...)
	assert.Equal(t, tAttrs, s.span.Attrs, "span attributes not set")

	builder.Options = []trace.SpanStartOption{
		trace.WithAttributes(attrs[0].Key.Bool(!attrs[0].Value.AsBool())),
	}

	s = builder.Build()
	s.SetAttributes(attrs...)
	assert.Equal(t, tAttrs, s.span.Attrs, "SpanAttributes did not override")
}

func TestSpanTracerProvider(t *testing.T) {
	var s span

	got := s.TracerProvider()
	assert.IsType(t, &tracerProvider{}, got)
}

type spanBuilder struct {
	Name        string
	NotSampled  bool
	SpanContext trace.SpanContext
	Options     []trace.SpanStartOption
}

func (b spanBuilder) Build() *span {
	tracer := new(tracer)
	s := &span{spanContext: b.SpanContext}
	s.sampled.Store(!b.NotSampled)
	s.traces, s.span = tracer.traces(
		b.Name,
		trace.NewSpanStartConfig(b.Options...),
		s.spanContext,
		trace.SpanContext{},
	)

	return s
}

func TestSpanConcurrentSafe(t *testing.T) {
	t.Parallel()

	const (
		nTracers   = 2
		nSpans     = 2
		nGoroutine = 10
	)

	runSpan := func(s trace.Span) <-chan struct{} {
		done := make(chan struct{})
		go func(span trace.Span) {
			defer close(done)

			var wg sync.WaitGroup
			for i := 0; i < nGoroutine; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()

					_ = s.IsRecording()
					_ = s.SpanContext()
					_ = s.TracerProvider()

					s.AddEvent("event")
					s.AddLink(trace.Link{})
					s.RecordError(errors.New("err"))
					s.SetStatus(codes.Error, "error")
					s.SetName("span" + strconv.Itoa(n))
					s.SetAttributes(attribute.Bool("key", true))

					s.End()
				}(i)
			}

			wg.Wait()
		}(s)
		return done
	}

	runTracer := func(tr trace.Tracer) <-chan struct{} {
		done := make(chan struct{})
		go func(tracer trace.Tracer) {
			defer close(done)

			ctx := context.Background()

			var wg sync.WaitGroup
			for i := 0; i < nSpans; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					_, s := tracer.Start(ctx, "span"+strconv.Itoa(n))
					<-runSpan(s)
				}(i)
			}

			wg.Wait()
		}(tr)
		return done
	}

	run := func(tp trace.TracerProvider) <-chan struct{} {
		done := make(chan struct{})
		go func(provider trace.TracerProvider) {
			defer close(done)

			var wg sync.WaitGroup
			for i := 0; i < nTracers; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					<-runTracer(provider.Tracer("tracer" + strconv.Itoa(n)))
				}(i)
			}

			wg.Wait()
		}(tp)
		return done
	}

	assert.NotPanics(t, func() {
		done0, done1 := run(TracerProvider()), run(TracerProvider())

		<-done0
		<-done1
	})
}
