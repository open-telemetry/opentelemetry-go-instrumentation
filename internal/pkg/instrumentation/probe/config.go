// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

type Config interface {
	// Package returns the name of the package instrumented by this [Probe].
	Package() string
}
