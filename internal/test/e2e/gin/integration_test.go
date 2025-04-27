// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package gin provides an integration test for the Gin probe.
package gin

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

	t.Run("Scope", func(t *testing.T) {
		assert.Equal(t, scopeName, scopes[0].Scope().Name(), "scope name")
	})

	serverS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
		return s.Name() == "GET" && s.Kind() == ptrace.SpanKindServer
	})
	require.NoError(t, err)
	t.Run("ServerSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, serverS.TraceID(), "trace ID")

		e2e.AssertSpanID(t, serverS.SpanID(), "span ID")

		attrs := e2e.AttributesMap(serverS.Attributes())
		assert.Equal(t, "GET", attrs["http.request.method"].Str(), "http.request.method")
		assert.Equal(t, "/hello-gin", attrs["url.path"].Str(), "http.url")
		assert.Equal(
			t,
			int64(200),
			attrs["http.response.status_code"].Int(),
			"http.response.status_code",
		)
		assert.Regexp(t, e2e.PortRE, attrs["network.peer.port"].AsString(), "network.protocol")
		assert.Equal(t, "localhost", attrs["server.address"].Str(), "server.address")
		assert.Equal(t, "1.1", attrs["network.protocol.version"].Str(), "network.protocol_version")
		assert.Equal(t, "::1", attrs["network.peer.address"].Str(), "network.peer.address")
	})

	clientS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
		return s.Name() == "GET" && s.Kind() == ptrace.SpanKindClient
	})
	require.NoError(t, err)
	t.Run("ClientSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, clientS.TraceID(), "trace ID")

		e2e.AssertSpanID(t, clientS.SpanID(), "span ID")

		attrs := e2e.AttributesMap(clientS.Attributes())
		assert.Equal(t, "GET", attrs["http.request.method"].Str(), "http.request.method")
		assert.Equal(t, "/hello-gin", attrs["url.path"].Str(), "http.url")
		assert.Equal(
			t,
			int64(200),
			attrs["http.response.status_code"].Int(),
			"http.response.status_code",
		)
		assert.Equal(t, "localhost", attrs["server.address"].Str(), "client.address")
		assert.Equal(t, int64(8080), attrs["server.port"].Int(), "server.port")
		assert.Equal(t, "1.1", attrs["network.protocol.version"].Str(), "network.protocol_version")
	})

	var clientSpanID [8]byte = clientS.SpanID()
	var serverParentSpanID [8]byte = serverS.ParentSpanID()
	assert.Equal(
		t,
		hex.EncodeToString(clientSpanID[:]),
		hex.EncodeToString(serverParentSpanID[:]),
		"client is parent of server",
	)

	var clientTraceID [16]byte = clientS.TraceID()
	var serverTraceID [16]byte = serverS.TraceID()
	assert.Equal(
		t,
		hex.EncodeToString(clientTraceID[:]),
		hex.EncodeToString(serverTraceID[:]),
		"client and server have the same trace ID",
	)
}
