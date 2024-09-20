// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

// copied from instrumentation.go.
func instResource() *resource.Resource {
	runVer := strings.TrimPrefix(runtime.Version(), "go")
	runName := runtime.Compiler
	if runName == "gc" {
		runName = "go"
	}
	runDesc := fmt.Sprintf(
		"go version %s %s/%s",
		runVer, runtime.GOOS, runtime.GOARCH,
	)

	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String("unknown_service"),
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetryDistroVersionKey.String("1.25.0"),
		semconv.ProcessRuntimeName(runName),
		semconv.ProcessRuntimeVersion(runVer),
		semconv.ProcessRuntimeDescription(runDesc),
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	)
}

func TestTrace(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Second)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(instResource()),
	)
	defer func() {
		err := tp.Shutdown(context.Background())
		assert.NoError(t, err)
	}()

	ctrl, err := NewController(slog.Default(), tp, "test")
	assert.NoError(t, err)

	spId, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	assert.NoError(t, err)
	trId, err := trace.TraceIDFromHex("00f067aa0ba902b700f067aa0ba902b7")
	assert.NoError(t, err)
	spanContext := trace.NewSpanContext(
		trace.SpanContextConfig{
			SpanID:     spId,
			TraceID:    trId,
			TraceFlags: 1,
		},
	)

	testCases := []struct {
		name     string
		event    *probe.Event
		expected tracetest.SpanStubs
	}{
		{
			name: "basic test span",
			event: &probe.Event{
				Package: "foo/bar",
				Kind:    trace.SpanKindClient,
				SpanEvents: []*probe.SpanEvent{
					{
						SpanName:     "testSpan",
						StartTime:    startTime,
						EndTime:      endTime,
						SpanContext:  &spanContext,
						TracerSchema: semconv.SchemaURL,
					},
				},
			},
			expected: tracetest.SpanStubs{
				{
					Name:      "testSpan",
					SpanKind:  trace.SpanKindClient,
					StartTime: startTime,
					EndTime:   endTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Scope{
						Name:      "go.opentelemetry.io/auto/foo/bar",
						Version:   "test",
						SchemaURL: semconv.SchemaURL,
					},
					InstrumentationScope: instrumentation.Scope{
						Name:      "go.opentelemetry.io/auto/foo/bar",
						Version:   "test",
						SchemaURL: semconv.SchemaURL,
					},
				},
			},
		},
		{
			name: "http/client",
			event: &probe.Event{
				Package: "net/http",
				Kind:    trace.SpanKindClient,
				SpanEvents: []*probe.SpanEvent{
					{
						SpanName:    "GET",
						StartTime:   startTime,
						EndTime:     endTime,
						SpanContext: &spanContext,
						Attributes: []attribute.KeyValue{
							semconv.HTTPRequestMethodKey.String("GET"),
							semconv.URLPath("/"),
							semconv.HTTPResponseStatusCodeKey.Int(200),
							semconv.ServerAddress("https://google.com"),
							semconv.ServerPort(8080),
						},
					},
				},
			},
			expected: tracetest.SpanStubs{
				{
					Name:      "GET",
					SpanKind:  trace.SpanKindClient,
					StartTime: startTime,
					EndTime:   endTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Scope{
						Name:    "go.opentelemetry.io/auto/net/http",
						Version: "test",
					},
					InstrumentationScope: instrumentation.Scope{
						Name:    "go.opentelemetry.io/auto/net/http",
						Version: "test",
					},
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String("GET"),
						semconv.URLPath("/"),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.ServerAddress("https://google.com"),
						semconv.ServerPort(8080),
					},
				},
			},
		},
		{
			name: "http/client with status code",
			event: &probe.Event{
				Package: "net/http",
				Kind:    trace.SpanKindClient,
				SpanEvents: []*probe.SpanEvent{
					{
						SpanName:    "GET",
						StartTime:   startTime,
						EndTime:     endTime,
						SpanContext: &spanContext,
						Attributes: []attribute.KeyValue{
							semconv.HTTPRequestMethodKey.String("GET"),
							semconv.URLPath("/"),
							semconv.HTTPResponseStatusCodeKey.Int(500),
							semconv.ServerAddress("https://google.com"),
							semconv.ServerPort(8080),
						},
						Status: probe.Status{Code: codes.Error},
					},
				},
			},
			expected: tracetest.SpanStubs{
				{
					Name:      "GET",
					SpanKind:  trace.SpanKindClient,
					StartTime: startTime,
					EndTime:   endTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Scope{
						Name:    "go.opentelemetry.io/auto/net/http",
						Version: "test",
					},
					InstrumentationScope: instrumentation.Scope{
						Name:    "go.opentelemetry.io/auto/net/http",
						Version: "test",
					},
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String("GET"),
						semconv.URLPath("/"),
						semconv.HTTPResponseStatusCodeKey.Int(500),
						semconv.ServerAddress("https://google.com"),
						semconv.ServerPort(8080),
					},
					Status: sdktrace.Status{Code: codes.Error},
				},
			},
		},
		{
			name: "otelglobal",
			event: &probe.Event{
				Kind: trace.SpanKindClient,
				SpanEvents: []*probe.SpanEvent{
					{
						SpanName:    "very important span",
						StartTime:   startTime,
						EndTime:     endTime,
						SpanContext: &spanContext,
						Attributes: []attribute.KeyValue{
							attribute.Int64("int.value", 42),
							attribute.String("string.value", "hello"),
							attribute.Float64("float.value", 3.14),
							attribute.Bool("bool.value", true),
						},
						Status:        probe.Status{Code: codes.Error, Description: "error description"},
						TracerName:    "user-tracer",
						TracerVersion: "v1",
						TracerSchema:  "user-schema",
					},
				},
			},
			expected: tracetest.SpanStubs{
				{
					Name:      "very important span",
					SpanKind:  trace.SpanKindClient,
					StartTime: startTime,
					EndTime:   endTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Scope{
						Name:      "user-tracer",
						Version:   "v1",
						SchemaURL: "user-schema",
					},
					InstrumentationScope: instrumentation.Scope{
						Name:      "user-tracer",
						Version:   "v1",
						SchemaURL: "user-schema",
					},
					Attributes: []attribute.KeyValue{
						attribute.Int64("int.value", 42),
						attribute.String("string.value", "hello"),
						attribute.Float64("float.value", 3.14),
						attribute.Bool("bool.value", true),
					},
					Status: sdktrace.Status{Code: codes.Error, Description: "error description"},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer exporter.Reset()
			ctrl.Trace(tt.event)
			tp.ForceFlush(context.Background())
			spans := exporter.GetSpans()
			assert.Equal(t, len(tt.expected), len(spans))

			// span contexts get modified by exporter, update expected with output
			for i, span := range spans {
				tt.expected[i].SpanContext = span.SpanContext
			}
			assert.Equal(t, tt.expected, spans)
		})
	}
}

func TestGetTracer(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(instResource()),
	)
	defer func() {
		err := tp.Shutdown(context.Background())
		assert.NoError(t, err)
	}()

	ctrl, err := NewController(slog.Default(), tp, "test")
	assert.NoError(t, err)

	t1 := ctrl.getTracer("foo/bar", "test", "v1", "schema")
	assert.Equal(t, t1, ctrl.tracersMap[tracerID{name: "test", version: "v1", schema: "schema"}])
	assert.Nil(t, ctrl.tracersMap[tracerID{name: "foo/bar", version: "v1", schema: "schema"}])

	t2 := ctrl.getTracer("net/http", "", "", "")
	assert.Equal(t, t2, ctrl.tracersMap[tracerID{name: "net/http", version: ctrl.version, schema: ""}])

	t3 := ctrl.getTracer("foo/bar", "test", "v1", "schema")
	assert.Same(t, t1, t3)

	t4 := ctrl.getTracer("net/http", "", "", "")
	assert.Same(t, t2, t4)
	assert.Equal(t, len(ctrl.tracersMap), 2)
}

type shutdownExporter struct {
	sdktrace.SpanExporter

	exported atomic.Uint32
	called   bool
}

// ExportSpans handles export of spans by storing them in memory.
func (e *shutdownExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.exported.Add(uint32(len(spans)))
	return nil
}

func (e *shutdownExporter) Shutdown(context.Context) error {
	e.called = true
	return nil
}

func TestShutdown(t *testing.T) {
	const nSpan = 10

	exporter := new(shutdownExporter)

	batcher := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(nSpan+1),
		sdktrace.WithBatchTimeout(nSpan+1),
		// Ensure we are checking Shutdown flushes the queue.
		sdktrace.WithBatchTimeout(time.Hour),
	)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(batcher))

	ctrl, err := NewController(slog.Default(), tp, "test")
	require.NoError(t, err)

	ctx := context.Background()
	tracer := tp.Tracer("test")
	for i := 0; i < nSpan; i++ {
		_, s := tracer.Start(ctx, "span"+strconv.Itoa(i))
		s.End()
	}

	require.NoError(t, ctrl.Shutdown(ctx))
	assert.True(t, exporter.called, "Exporter not shutdown")
	assert.Equal(t, uint32(nSpan), exporter.exported.Load(), "Pending spans not flushed")
}
