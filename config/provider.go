// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// InstrumentationLibraryID is used to identify an instrumentation library.
type InstrumentationLibraryID struct {
	// Name of the instrumentation pkg (e.g. "net/http").
	InstrumentedPkg string
	// SpanKind is the relevant span kind for the instrumentation.
	// This can be used to configure server-only, client-only spans.
	// If not set, the identifier is assumed to be applicable to all span kinds relevant to the instrumented package.
	SpanKind trace.SpanKind
}

// InstrumentationLibrary is used to configure instrumentation for a specific library.
type InstrumentationLibrary struct {
	// TracesEnabled determines whether traces are enabled for the instrumentation library.
	// if nil - take DefaultTracesDisabled value.
	TracesEnabled *bool
}

// InstrumentationConfig is used to configure instrumentation.
type InstrumentationConfig struct {
	// InstrumentationLibraryConfigs defines library-specific configuration.
	// If a package is referenced by more than one key, the most specific key is used.
	// For example, if ("net/http", unspecified) and ("net/http", client) are both present,
	// the configuration for ("net/http", client) is used for client spans and the configuration for ("net/http", unspecified) is used for server spans.
	InstrumentationLibraryConfigs map[InstrumentationLibraryID]InstrumentationLibrary

	// DefaultTracesDisabled determines whether traces are disabled by default.
	// If set to true, traces are disabled by default for all libraries, unless the library is explicitly enabled.
	// If set to false, traces are enabled by default for all libraries, unless the library is explicitly disabled.
	// default is false - traces are enabled by default.
	DefaultTracesDisabled bool
}

// Provider provides the initial configuration and updates to the instrumentation configuration.
type Provider interface {
	// InitialConfig returns the initial instrumentation configuration.
	InitialConfig(ctx context.Context) InstrumentationConfig
	// Watch returns a channel that receives updates to the instrumentation configuration.
	Watch() <-chan InstrumentationConfig
	// Shutdown releases any resources held by the provider.
	Shutdown(ctx context.Context) error
}

type noopProvider struct{}

// NewNoopProvider returns a provider that does not provide any updates and provide the default configuration as the initial one.
func NewNoopProvider() Provider {
	return &noopProvider{}
}

func (p *noopProvider) InitialConfig(_ context.Context) InstrumentationConfig {
	return InstrumentationConfig{}
}

func (p *noopProvider) Watch() <-chan InstrumentationConfig {
	c := make(chan InstrumentationConfig)
	close(c)
	return c
}

func (p *noopProvider) Shutdown(_ context.Context) error {
	return nil
}
