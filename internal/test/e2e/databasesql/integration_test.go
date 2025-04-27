// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package databasesql provides an integration test for the database/sql probe.
package databasesql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

// scopeName defines the instrumentation scope name used in the trace.
const scopeName = "go.opentelemetry.io/auto/database/sql"

// queryTextKey defines the attribute key for the SQL query text.
const queryTextKey = string(semconv.DBQueryTextKey)

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

	runS, err := e2e.SpanByName(scopes, "run")
	require.NoError(t, err)

	selectS, err := e2e.SpanByName(scopes, "SELECT contacts")
	require.NoError(t, err)
	t.Run("SELECT", verify(selectS, runS, "SELECT * FROM contacts"))

	insertS, err := e2e.SpanByName(scopes, "INSERT contacts")
	require.NoError(t, err)
	t.Run("INSERT", verify(insertS, runS, "INSERT INTO contacts (first_name) VALUES ('Mike')"))

	updateS, err := e2e.SpanByName(scopes, "UPDATE contacts")
	require.NoError(t, err)
	t.Run(
		"UPDATE",
		verify(updateS, runS, "UPDATE contacts SET last_name = 'Santa' WHERE first_name = 'Mike'"),
	)

	deleteS, err := e2e.SpanByName(scopes, "DELETE contacts")
	require.NoError(t, err)
	t.Run("DELETE", verify(deleteS, runS, "DELETE FROM contacts WHERE first_name = 'Mike'"))

	query := "DROP TABLE contacts"
	dropS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
		name := s.Name()
		v, ok := e2e.AttributesMap(s.Attributes())[queryTextKey]
		return name == "DB" && ok && v.Str() == query
	})
	require.NoError(t, err)
	t.Run("DROP", verify(dropS, runS, query))

	query = "syntax error"
	errS, err := e2e.SelectSpan(scopes, func(s ptrace.Span) bool {
		name := s.Name()
		v, ok := e2e.AttributesMap(s.Attributes())[queryTextKey]
		return name == "DB" && ok && v.Str() == query
	})
	require.NoError(t, err)
	t.Run("ERROR", verify(errS, runS, query))
}

func verify(span, parent ptrace.Span, query string) func(t *testing.T) {
	return func(t *testing.T) {
		e2e.AssertTraceID(t, span.TraceID(), "trace ID")
		e2e.AssertSpanID(t, span.SpanID(), "span ID")
		assert.Equal(t, parent.SpanID(), span.ParentSpanID(), "parent span ID")

		attrs := e2e.AttributesMap(span.Attributes())

		assert.Equal(t, query, attrs[queryTextKey].Str(), queryTextKey)
	}
}
