// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package autosdk provides an integration test for the autosdk probe.
package autosdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "go.opentelemetry.io/auto/internal/test/e2e/autosdk"

// Y2K (January 1, 2000).
var y2k = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

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

	mainSpan, err := e2e.SpanByName(scopes, "main")
	require.NoError(t, err)
	t.Run("MainSpan", func(t *testing.T) {
		e2e.AssertTraceID(t, mainSpan.TraceID(), "trace ID")

		e2e.AssertSpanID(t, mainSpan.SpanID(), "span ID")

		assert.Equal(t, y2k, mainSpan.StartTimestamp().AsTime(), "start time")
		assert.Equal(t, y2k.Add(5*time.Second), mainSpan.EndTimestamp().AsTime(), "end time")

		assert.Equal(t, ptrace.SpanKindInternal, mainSpan.Kind(), "span kind")

		status := mainSpan.Status()
		assert.Equal(t, ptrace.StatusCodeError, status.Code(), "status code")
		assert.Equal(t, "application error", status.Message(), "status message")

		t.Run("Event", func(t *testing.T) {
			event, err := e2e.EventByName(mainSpan, "exception")
			require.NoError(t, err)

			assert.Equal(t, y2k.Add(2*time.Second), event.Timestamp().AsTime(), "event time")

			attrs := e2e.AttributesMap(event.Attributes())
			assert.Equal(t, int64(11), attrs["impact"], "event attribute impact")
			assert.Equal(
				t,
				"*errors.errorString",
				attrs["exception.type"],
				"event attribute exception type",
			)
			assert.Equal(
				t,
				"broken",
				attrs["exception.message"],
				"event attribute exception message",
			)
			assert.NotEmpty(
				t,
				attrs["exception.stacktrace"],
				"event attribute exception stacktrace",
			)
		})
	})

	sigSpan, err := e2e.SpanByName(scopes, "sig")
	require.NoError(t, err)
	t.Run("SigSpan", func(t *testing.T) {
		assert.Equal(t, mainSpan.TraceID(), sigSpan.TraceID(), "trace ID")

		e2e.AssertSpanID(t, sigSpan.SpanID(), "span ID")

		assert.Equal(t, mainSpan.SpanID(), sigSpan.ParentSpanID(), "parent span ID")

		assert.Equal(
			t,
			y2k.Add(10*time.Microsecond),
			sigSpan.StartTimestamp().AsTime(),
			"start time",
		)
		assert.Equal(t, y2k.Add(110*time.Microsecond), sigSpan.EndTimestamp().AsTime(), "end time")

		assert.Equal(t, ptrace.SpanKindInternal, sigSpan.Kind(), "span kind")
	})

	runSpan, err := e2e.SpanByName(scopes, "Run")
	require.NoError(t, err)
	t.Run("RunSpan", func(t *testing.T) {
		assert.Equal(t, mainSpan.TraceID(), runSpan.TraceID(), "trace ID")

		e2e.AssertSpanID(t, runSpan.SpanID(), "span ID")

		assert.Equal(t, mainSpan.SpanID(), runSpan.ParentSpanID(), "parent span ID")

		assert.Equal(
			t,
			y2k.Add(500*time.Microsecond),
			runSpan.StartTimestamp().AsTime(),
			"start time",
		)
		assert.Equal(t, y2k.Add(1*time.Second), runSpan.EndTimestamp().AsTime(), "end time")

		assert.Equal(t, ptrace.SpanKindServer, runSpan.Kind(), "span kind")

		attrs := e2e.AttributesMap(runSpan.Attributes())
		assert.Equal(t, "Alice", attrs["user"], "user attribute")
		assert.Equal(t, true, attrs["admin"], "admin attribute")

		t.Run("Link", func(t *testing.T) {
			if !assert.Equal(t, 1, runSpan.Links().Len(), "number of links") {
				return
			}

			link := runSpan.Links().At(0)
			assert.Equal(t, sigSpan.TraceID(), link.TraceID(), "link trace ID")
			assert.Equal(t, sigSpan.SpanID(), link.SpanID(), "link span ID")

			attrs := e2e.AttributesMap(link.Attributes())
			assert.Equal(t, "Hello World", attrs["data"], "link attribute data")
		})
	})
}
