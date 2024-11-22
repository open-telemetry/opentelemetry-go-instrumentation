// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"log/slog"
)

// BaseProbe is the most basic type of Probe. It must support configuration,
// loading and attaching eBPF programs, running, and closing.
// All Probes must implement this Base functionality.
type BaseProbe interface {
	// ID is a unique identifier for the Probe.
	// All types of Probes must provide a unique identifier for the framework.
	ID() ID

	// GetLogger returns an *slog.Logger for this Probe.
	GetLogger() *slog.Logger

	// Load loads the eBPF programs and maps required by the Probe into memory.
	// The specific types of programs and maps are implemented by the Probe.
	Load() error

	// Attach attaches the eBPF programs to trigger points in the process.
	// The specific attachment points are implemented by the Probe.
	Attach() error

	// ApplyConfig updates the Probe's current Config with the provided Config
	// interface. It is up to the Probe to implement type conversion to any custom
	// config formats it defines, to support options specific to the Probe.
	ApplyConfig(Config) error
}

// RunnableProbe is a Probe that Runs.
type RunnableProbe interface {
	BaseProbe

	// Run starts the Probe.
	Run()

	// Close stops the Probe.
	Close() error
}

// TracingProbe is a RunnableProbe meant specifically for trace telemetry.
type TracingProbe interface {
	RunnableProbe

	// TraceConfig provides the TracingConfig for the Probe.
	TraceConfig() *TracingConfig
}

// GoLibraryTelemetryProbe is a RunnableProbe that is bound to a single library
// in a single target executable.
type GoLibraryTelemetryProbe interface {
	RunnableProbe

	// Manifest returns the Probe's instrumentation Manifest. This includes all
	// the information about any packages the Probe instruments.
	Manifest() Manifest

	// TargetConfig returns the Probe's TargetExecutableConfig containing
	// information about the process the Probe observes.
	TargetConfig() *TargetExecutableConfig
}

// Config represents the config for a Probe.
// There are currently no default options for Probes, however Probes may
// define their own config structs that the Probe must cast itself when
// implementing ApplyConfig().
type Config interface{}
