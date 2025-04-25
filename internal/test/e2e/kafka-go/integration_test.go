// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package kafka provides an integration test for the Kafka probe.
package kafka

import (
	"testing"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

func TestIntegration(t *testing.T) {
	e2e.TestIntegration(t, "./cmd", ".")
}
