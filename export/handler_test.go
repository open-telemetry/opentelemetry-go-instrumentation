// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package export_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/export"
)

func TestTelemetrySchemaURL(t *testing.T) {
	tel := new(export.Telemetry)

	assert.Empty(t, tel.SchemaURL(), "new Telemetry")

	const url = "test"
	tel.SetSchemaURL(url)
	assert.Equal(t, url, tel.SchemaURL())
}

func TestTelemetryScope(t *testing.T) {
	tel := new(export.Telemetry)

	assert.False(t, tel.HasScope(), "new Telemetry")

	scope := tel.Scope()
	assert.True(t, tel.HasScope())

	const name = "test"
	scope.SetName(name)
	assert.Equal(t, scope, tel.Scope())
}

func TestTelemetrySpans(t *testing.T) {
	tel := new(export.Telemetry)

	assert.False(t, tel.HasSpans(), "new Telemetry")

	spans := tel.Spans()
	assert.False(t, tel.HasSpans(), "len == 0")

	_ = spans.AppendEmpty()
	assert.True(t, tel.HasSpans())
	assert.Equal(t, spans, tel.Spans())
}

func TestTelemetryMetrics(t *testing.T) {
	tel := new(export.Telemetry)

	assert.False(t, tel.HasMetrics(), "new Telemetry")

	spans := tel.Metrics()
	assert.False(t, tel.HasMetrics(), "len == 0")

	_ = spans.AppendEmpty()
	assert.True(t, tel.HasMetrics())
	assert.Equal(t, spans, tel.Metrics())
}

func TestTelemetryLogs(t *testing.T) {
	tel := new(export.Telemetry)

	assert.False(t, tel.HasLogs(), "new Telemetry")

	spans := tel.Logs()
	assert.False(t, tel.HasLogs(), "len == 0")

	_ = spans.AppendEmpty()
	assert.True(t, tel.HasLogs())
	assert.Equal(t, spans, tel.Logs())
}
