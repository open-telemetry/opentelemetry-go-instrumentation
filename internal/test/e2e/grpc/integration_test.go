// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package grpc provides an integration test for the gRPC probe.
package grpc

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "go.opentelemetry.io/auto/google.golang.org/grpc"

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

	t.Run("Scope", func(t *testing.T) {
		assert.Equal(t, scopeName, scopes[0].Scope().Name(), "scope name")
	})

	var count int
	for _, scope := range scopes {
		spans := scope.Spans()
		for i := range spans.Len() {
			span := spans.At(i)
			if span.Kind() != ptrace.SpanKindServer {
				continue
			}
			count++
			t.Run("ServerSpan/"+strconv.Itoa(count), func(t *testing.T) {
				assert.Equal(t, "/helloworld.Greeter/SayHello", span.Name(), "span name")

				e2e.AssertTraceID(t, span.TraceID(), "trace ID")
				e2e.AssertSpanID(t, span.SpanID(), "span ID")
				e2e.AssertSpanID(t, span.ParentSpanID(), "parent span ID")

				attrs := e2e.AttributesMap(span.Attributes())
				assert.Equal(t, "grpc", attrs["rpc.system"], "rpc.system")
				assert.Equal(
					t,
					"/helloworld.Greeter/SayHello",
					attrs["rpc.service"],
					"rpc.service",
				)
				assert.Equal(t, "127.0.0.1", attrs["server.address"], "server.address")
				assert.Equal(t, int64(1701), attrs["server.port"], "server.port")

				code, ok := attrs["rpc.grpc.status_code"]
				assert.True(t, ok, "has rpc.grpc.status_code attribute")
				if v, ok := code.(int64); ok && v != 0 {
					assert.Equal(t, int64(12), v, "error code value")
				}
			})
		}
	}
	assert.GreaterOrEqual(t, count, 2, "at least two server span")

	for i := range 2 {
		t.Run("Heritage/"+strconv.Itoa(i), func(t *testing.T) {
			otelScope := "go.opentelemetry.io/auto/internal/test/e2e/grpc"
			otelScopes := e2e.ScopeSpansByName(traces, otelScope)
			otelS, err := e2e.SelectSpan(otelScopes, nameN("SayHello", i))

			tid := [16]byte(otelS.TraceID())
			otelTID := hex.EncodeToString(tid[:])

			// Received order of the spans is not guaranteed. Find the related
			// client and server spans by their expected trace ID instead.

			require.NoError(t, err)
			cS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
				if s.Kind() != ptrace.SpanKindClient {
					return false
				}

				b := [16]byte(s.TraceID())
				tid := hex.EncodeToString(b[:])
				return tid == otelTID
			})
			require.NoError(t, err)

			sS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
				if s.Kind() != ptrace.SpanKindServer {
					return false
				}

				b := [16]byte(s.TraceID())
				tid := hex.EncodeToString(b[:])
				return tid == otelTID
			})
			require.NoError(t, err)

			assert.Equal(t, sS.SpanID(), otelS.ParentSpanID(), "server-otel parent span ID")
			assert.Equal(t, cS.SpanID(), sS.ParentSpanID(), "client-server parent span ID")
		})
	}

	count = 0
	for _, scope := range scopes {
		spans := scope.Spans()
		for i := range spans.Len() {
			span := spans.At(i)
			if span.Kind() != ptrace.SpanKindClient {
				continue
			}
			count++
			t.Run("ClientSpan/"+strconv.Itoa(count), func(t *testing.T) {
				assert.Equal(t, "/helloworld.Greeter/SayHello", span.Name(), "span name")

				e2e.AssertTraceID(t, span.TraceID(), "trace ID")
				e2e.AssertSpanID(t, span.SpanID(), "span ID")
				e2e.AssertSpanID(t, span.ParentSpanID(), "parent span ID")

				attrs := e2e.AttributesMap(span.Attributes())
				assert.Equal(t, "grpc", attrs["rpc.system"], "rpc.system")
				assert.Equal(
					t,
					"/helloworld.Greeter/SayHello",
					attrs["rpc.service"],
					"rpc.service",
				)
				assert.Equal(t, int64(1701), attrs["server.port"], "server.port")

				code, ok := attrs["rpc.grpc.status_code"]
				assert.True(t, ok, "has rpc.grpc.status_code attribute")
				if v, ok := code.(int64); ok && v != 0 && v != 12 && v != 14 {
					t.Errorf("unexpected rpc.grpc.status_code value: %d", v)
				}

				msg := span.Status().Message()
				conErr := "connection error: desc = \"transport: Error while dialing: dial tcp [::1]:1701: connect: connection refused\""
				unimpl := "unimplmented"
				if msg != "" && msg != conErr && msg != unimpl {
					t.Errorf("unexpected status message: %s", msg)
				}
			})
		}
	}
	assert.GreaterOrEqual(t, count, 3, "at least three client span")
}

func nameN(name string, n int) func(ptrace.Span) bool {
	var count int
	return func(span ptrace.Span) bool {
		if span.Name() == name {
			if count == n {
				return true
			}
			count++
		}
		return false
	}
}
