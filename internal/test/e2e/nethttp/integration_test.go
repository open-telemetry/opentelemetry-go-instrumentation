// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package nethttp provides an integration test for the net/http probe.
package nethttp

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "go.opentelemetry.io/auto/net/http"

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

	for i, scope := range scopes {
		t.Run("Scope/"+strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, scopeName, scope.Scope().Name(), "scope name")
			assert.Equal(t, semconv.SchemaURL, scope.SchemaUrl(), "schema URL")
		})
	}

	serverSpan, err := e2e.SpanByName(scopes, "GET /hello/{id}")
	require.NoError(t, err)

	t.Run("ServerSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, serverSpan.TraceID(), "trace ID")
		e2e.AssertSpanID(t, serverSpan.SpanID(), "span ID")
		e2e.AssertSpanID(t, serverSpan.ParentSpanID(), "parent span ID")

		assert.Equal(t, "GET /hello/{id}", serverSpan.Name(), "span name")

		assert.Equal(t, ptrace.SpanKindServer, serverSpan.Kind(), "span kind")

		attrs := e2e.AttributesMap(serverSpan.Attributes())
		assert.Equal(t, "GET", attrs[string(semconv.HTTPRequestMethodKey)], "HTTP method")
		assert.Equal(t, "/hello/42", attrs[string(semconv.URLPathKey)], "URL path")
		assert.Equal(
			t,
			int64(200),
			attrs[string(semconv.HTTPResponseStatusCodeKey)],
			"HTTP status code",
		)
		assert.Regexp(
			t,
			e2e.PortRE,
			attrs[string(semconv.NetworkPeerPortKey)],
			"network peer port",
		)
		assert.Equal(
			t,
			"localhost",
			attrs[string(semconv.ServerAddressKey)],
			"server address",
		)
		assert.Equal(
			t,
			"1.1",
			attrs[string(semconv.NetworkProtocolVersionKey)],
			"network protocol version",
		)
		assert.Equal(
			t,
			"::1",
			attrs[string(semconv.NetworkPeerAddressKey)],
			"network peer address",
		)
		assert.Equal(t, "/hello/{id}", attrs[string(semconv.HTTPRouteKey)], "HTTP route")
	})

	clientSpan, err := e2e.SpanByName(scopes, "GET")
	require.NoError(t, err)

	t.Run("ClientSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, clientSpan.TraceID(), "trace ID")
		e2e.AssertSpanID(t, clientSpan.SpanID(), "span ID")

		assert.Equal(t, "GET", clientSpan.Name(), "span name")

		assert.Equal(t, ptrace.SpanKindClient, clientSpan.Kind(), "span kind")

		attrs := e2e.AttributesMap(clientSpan.Attributes())
		assert.Equal(t, "/hello/42", attrs[string(semconv.URLPathKey)], "URL path")
		assert.Equal(
			t,
			int64(200),
			attrs[string(semconv.HTTPResponseStatusCodeKey)],
			"HTTP status code",
		)
		assert.Equal(
			t,
			"localhost",
			attrs[string(semconv.ServerAddressKey)],
			"server address",
		)
		assert.Equal(t, int64(8080), attrs[string(semconv.ServerPortKey)], "server port")
		assert.Equal(
			t,
			"1.1",
			attrs[string(semconv.NetworkProtocolVersionKey)],
			"network protocol version",
		)
		url := "http://user@localhost:8080/hello/42?query=true#fragment"
		assert.Equal(t, url, attrs[string(semconv.URLFullKey)], "full URL")
	})

	tIDByte := [16]byte(serverSpan.TraceID())
	serverTID := hex.EncodeToString(tIDByte[:])
	tIDByte = [16]byte(clientSpan.TraceID())
	clientTID := hex.EncodeToString(tIDByte[:])
	assert.Equal(t, serverTID, clientTID, "trace ID")

	sIDByte := [8]byte(serverSpan.ParentSpanID())
	serverPID := hex.EncodeToString(sIDByte[:])
	sIDByte = [8]byte(clientSpan.SpanID())
	clientSID := hex.EncodeToString(sIDByte[:])
	assert.Equal(t, clientSID, serverPID, "client should be parent of server span")
}
