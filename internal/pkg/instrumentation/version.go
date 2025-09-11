// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package instrumentation provides functionality to manage instrumentation
// using eBPF for Go programs.
package instrumentation

const (
	// Name is used for `telemetry.distro.name` resource attribute.
	Name = "opentelemetry-go-instrumentation"
	// Version is the current release version of OpenTelemetry Go auto-instrumentation in use.
	Version = "v0.23.0"
)
