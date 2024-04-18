// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opentelemetry

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/stdr"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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
		semconv.TelemetryAutoVersionKey.String("1.25.0"),
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
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))

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

	ctrl, err := NewController(logger, tp, "test")
	assert.NoError(t, err)

	convertedStartTime := ctrl.convertTime(startTime.Unix())
	convertedEndTime := ctrl.convertTime(endTime.Unix())

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
						SpanName:    "testSpan",
						StartTime:   startTime.Unix(),
						EndTime:     endTime.Unix(),
						SpanContext: &spanContext,
					},
				},
			},
			expected: tracetest.SpanStubs{
				{
					Name:      "testSpan",
					SpanKind:  trace.SpanKindClient,
					StartTime: convertedStartTime,
					EndTime:   convertedEndTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Library{
						Name:    "go.opentelemetry.io/auto/foo/bar",
						Version: "test",
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
						StartTime:   startTime.Unix(),
						EndTime:     endTime.Unix(),
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
					StartTime: convertedStartTime,
					EndTime:   convertedEndTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Library{
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
						StartTime:   startTime.Unix(),
						EndTime:     endTime.Unix(),
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
					StartTime: convertedStartTime,
					EndTime:   convertedEndTime,
					Resource:  instResource(),
					InstrumentationLibrary: instrumentation.Library{
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
