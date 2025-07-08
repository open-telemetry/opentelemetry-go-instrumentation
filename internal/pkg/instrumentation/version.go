// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package instrumentation provides functionality to manage instrumentation
// using eBPF for Go programs.
package instrumentation

const (
	// DistributionName is used for `telemetry.distro.name` resource attribute.
	DistributionName = "opentelemetry-go-instrumentation"
	// DistributionVersion is the current release version of OpenTelemetry Go auto-instrumentation in use.
	DistributionVersion = "v0.22.1"
)
