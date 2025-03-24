// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
)

// LibraryID is used to identify an instrumentation library.
type LibraryID struct {
	// Name of the instrumentation pkg (e.g. "net/http").
	InstrumentedPkg string
	// SpanKind is the relevant span kind for the instrumentation.
	// This can be used to configure server-only, client-only spans.
	// If not set, the identifier is assumed to be applicable to all span kinds relevant to the instrumented package.
	SpanKind trace.SpanKind
}

// Library is used to configure instrumentation for a specific library.
type Library struct {
	// TracesEnabled determines whether traces are enabled for the instrumentation library.
	// if nil - take DefaultTracesDisabled value.
	TracesEnabled *bool
}

// Config is used to configure instrumentation.
type Config struct {
	// InstrumentationLibraryConfigs defines library-specific configuration.
	// If a package is referenced by more than one key, the most specific key is used.
	// For example, if ("net/http", unspecified) and ("net/http", client) are both present,
	// the configuration for ("net/http", client) is used for client spans and the configuration for ("net/http", unspecified) is used for server spans.
	InstrumentationLibraryConfigs map[LibraryID]Library

	// DefaultTracesDisabled determines whether traces are disabled by default.
	// If set to true, traces are disabled by default for all libraries, unless the library is explicitly enabled.
	// If set to false, traces are enabled by default for all libraries, unless the library is explicitly disabled.
	// default is false - traces are enabled by default.
	DefaultTracesDisabled bool

	SamplingConfig *sampling.Config
}

// ConfigProvider provides the initial configuration and updates to the instrumentation configuration.
type ConfigProvider interface {
	// InitialConfig returns the initial instrumentation configuration.
	InitialConfig(ctx context.Context) Config
	// Watch returns a channel that receives updates to the instrumentation configuration.
	Watch() <-chan Config
	// Shutdown releases any resources held by the provider.
	// It is an error to send updates after Shutdown is called.
	Shutdown(ctx context.Context) error
}

type noopProvider struct {
	SamplingConfig *sampling.Config
}

// NewNoopConfigProvider returns a provider that does not provide any updates
// and provide the default configuration as the initial one.
func NewNoopConfigProvider(sc *sampling.Config) ConfigProvider {
	return &noopProvider{SamplingConfig: sc}
}

func (p *noopProvider) InitialConfig(_ context.Context) Config {
	return Config{
		SamplingConfig: p.SamplingConfig,
	}
}

func (p *noopProvider) Watch() <-chan Config {
	c := make(chan Config)
	close(c)
	return c
}

func (p *noopProvider) Shutdown(_ context.Context) error {
	return nil
}
