// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
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
	startTime := time.Unix(0, 0).UTC()
	endTime := time.Unix(1, 0).UTC()

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

	ctrl, err := NewController(slog.Default(), tp)
	assert.NoError(t, err)

	spId, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	assert.NoError(t, err)
	trId, err := trace.TraceIDFromHex("00f067aa0ba902b700f067aa0ba902b7")
	assert.NoError(t, err)

	testCases := []struct {
		name     string
		traces   ptrace.ScopeSpans
		expected tracetest.SpanStubs
	}{
		{
			name: "basic test span",
			traces: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.SetSchemaUrl(semconv.SchemaURL)

				scope := ss.Scope()
				scope.SetName("go.opentelemetry.io/auto/foo/bar")
				scope.SetVersion("test")

				span := ss.Spans().AppendEmpty()
				span.SetName("testSpan")
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				return ss
			}(),
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
			traces: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()

				scope := ss.Scope()
				scope.SetName("go.opentelemetry.io/auto/net/http")
				scope.SetVersion("test")

				span := ss.Spans().AppendEmpty()
				span.SetName("GET")
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/"),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.ServerAddress("https://google.com"),
					semconv.ServerPort(8080),
				)

				return ss
			}(),
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
			traces: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()

				scope := ss.Scope()
				scope.SetName("go.opentelemetry.io/auto/net/http")
				scope.SetVersion("test")

				span := ss.Spans().AppendEmpty()
				span.SetName("GET")
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
				span.Status().SetCode(ptrace.StatusCodeError)

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/"),
					semconv.HTTPResponseStatusCodeKey.Int(500),
					semconv.ServerAddress("https://google.com"),
					semconv.ServerPort(8080),
				)

				return ss
			}(),
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
			traces: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.SetSchemaUrl("user-schema")

				scope := ss.Scope()
				scope.SetName("user-tracer")
				scope.SetVersion("v1")

				span := ss.Spans().AppendEmpty()
				span.SetName("very important span")
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
				span.Status().SetCode(ptrace.StatusCodeError)
				span.Status().SetMessage("error description")

				utils.Attributes(
					span.Attributes(),
					attribute.Int64("int.value", 42),
					attribute.String("string.value", "hello"),
					attribute.Float64("float.value", 3.14),
					attribute.Bool("bool.value", true),
				)

				return ss
			}(),
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
			ctrl.Trace(tt.traces)
			tp.ForceFlush(context.Background())
			spans := exporter.GetSpans()
			assert.Len(t, spans, len(tt.expected))

			// span contexts get modified by exporter, update expected with output
			for i, span := range spans {
				tt.expected[i].SpanContext = span.SpanContext
			}
			assert.Equal(t, tt.expected, spans)
		})
	}
}

type shutdownExporter struct {
	sdktrace.SpanExporter

	exported atomic.Uint32
	called   bool
}

// ExportSpans handles export of spans by storing them in memory.
func (e *shutdownExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	n := len(spans)
	if n < 0 || n > math.MaxUint32 {
		return fmt.Errorf("invalid span length: %d", n)
	}
	e.exported.Add(uint32(n)) // nolint: gosec  // Bound checked
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

	ctrl, err := NewController(slog.Default(), tp)
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

func TestControllerTraceConcurrentSafe(t *testing.T) {
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

	ctrl, err := NewController(slog.Default(), tp)
	assert.NoError(t, err)

	const goroutines = 10

	var wg sync.WaitGroup
	for n := 0; n < goroutines; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			data := ptrace.NewScopeSpans()
			data.Scope().SetName(fmt.Sprintf("tracer-%d", n%(goroutines/2)))
			data.Scope().SetVersion("v1")
			data.SetSchemaUrl("url")
			data.Spans().AppendEmpty().SetName("test")
			ctrl.Trace(data)
		}()
	}

	wg.Wait()
}
