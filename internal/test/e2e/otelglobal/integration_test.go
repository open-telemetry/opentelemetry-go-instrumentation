// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelglobal provides an integration test for the global OTel trace
// probe.
package otelglobal

import (
	"testing"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

func TestIntegration(t *testing.T) {
	e2e.TestIntegration(t, "./cmd", ".")
}
