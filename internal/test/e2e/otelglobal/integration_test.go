// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package autosdk provides an integration test for the autosdk probe.
package autosdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "trace-example"

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
		assert.Equal(t, "v1.23.42", scopes[0].Scope().Version(), "scope version")
	})

	t.Run("SchemaURL", func(t *testing.T) {
		assert.Equal(t, "https://some_schema", scopes[0].SchemaUrl())
	})

	parentSpan, err := e2e.SpanByName(scopes, "parent")
	require.NoError(t, err)
	t.Run("ParentSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, parentSpan.TraceID(), "trace ID")
		e2e.AssertSpanID(t, parentSpan.SpanID(), "span ID")

		assert.Equal(t, ptrace.SpanKindServer, parentSpan.Kind(), "span kind")

		status := parentSpan.Status()
		assert.Equal(t, ptrace.StatusCodeOk, status.Code(), "status code")

		attrs := e2e.AttributesMap(parentSpan.Attributes())
		assert.Equal(t, int64(42), attrs["int_key"].Int(), "int_key")
		assert.Equal(t, "forty-two", attrs["string_key"].Str(), "string_key")
		assert.True(t, attrs["bool_key"].Bool(), "bool_key")
		assert.Equal(t, 42.3, attrs["float_key"].Double(), "float_key")
	})

	childSpan, err := e2e.SpanByName(scopes, "child override")
	require.NoError(t, err)
	t.Run("ChildSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, childSpan.TraceID(), "trace ID")
		e2e.AssertSpanID(t, childSpan.SpanID(), "span ID")
		assert.Equal(t, parentSpan.SpanID(), childSpan.ParentSpanID(), "parent span ID")

		assert.Equal(t, ptrace.SpanKindInternal, childSpan.Kind(), "span kind")

		status := childSpan.Status()
		assert.Equal(t, ptrace.StatusCodeError, status.Code(), "status code")
		assert.Equal(t, "i deleted the prod db sry", status.Message(), "status message")

		attrs := e2e.AttributesMap(childSpan.Attributes())
		assert.Equal(t, "inner.value", attrs["inner.key"].Str(), "inner.key")
		assert.True(t, attrs["cat.on_keyboard"].Bool(), "cat.on_keyboard")

		t.Run("Event", func(t *testing.T) {
			event, err := e2e.EventByName(childSpan, "exception")
			require.NoError(t, err)

			attrs := e2e.AttributesMap(event.Attributes())
			assert.Equal(
				t,
				"*errors.errorString",
				attrs["exception.type"].Str(),
				"event attribute exception type",
			)
			assert.Equal(
				t,
				"i deleted the prod db sry",
				attrs["exception.message"].Str(),
				"event attribute exception message",
			)
		})

		t.Run("Link", func(t *testing.T) {
			if !assert.Equal(t, 1, childSpan.Links().Len(), "number of links") {
				return
			}

			link := childSpan.Links().At(0)
			assert.Equal(t, pcommon.TraceID{0x2}, link.TraceID(), "link trace ID")
			assert.Equal(t, pcommon.SpanID{0x1}, link.SpanID(), "link span ID")
		})
	})
}
