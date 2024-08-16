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
	// Enabled determines whether instrumentation is enabled for the library.
	Enabled bool
}

// InstrumentationConfig is used to configure instrumentation.
type InstrumentationConfig struct {
	InstrumentationLibraryConfigs map[InstrumentationLibraryID]InstrumentationLibrary
	// DefaultDisabled determines whether instrumentation is disabled by default.
	// If set to true, instrumentation is disabled by default for all libraries, unless the library is explicitly enabled.
	// If set to false, instrumentation is enabled by default for all libraries, unless the library is explicitly disabled.
	DefaultDisabled bool
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
