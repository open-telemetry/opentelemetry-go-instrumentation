// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package grpc provides an integration test for the gRPC probe.
package grpc

import (
	"testing"

	"go.opentelemetry.io/auto/internal/test/e2e"
)

func TestIntegration(t *testing.T) {
	e2e.TestIntegration(t, "./cmd", ".")
}
