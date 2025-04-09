// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation"
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

	// Sampler is used to determine whether a trace should be sampled and exported.
	Sampler Sampler
}

// ConfigProvider provides the initial configuration and updates to the instrumentation configuration.
type ConfigProvider interface {
	// InitialConfig returns the initial instrumentation configuration.
	InitialConfig(ctx context.Context) InstrumentationConfig
	// Watch returns a channel that receives updates to the instrumentation configuration.
	Watch() <-chan InstrumentationConfig
	// Shutdown releases any resources held by the provider.
	// It is an error to send updates after Shutdown is called.
	Shutdown(ctx context.Context) error
}

type noopProvider struct {
	Sampler Sampler
}

// NewNoopProvider returns a provider that does not provide any updates and provide the default configuration as the initial one.
func newNoopConfigProvider(s Sampler) ConfigProvider {
	return &noopProvider{Sampler: s}
}

func (p *noopProvider) InitialConfig(_ context.Context) InstrumentationConfig {
	return InstrumentationConfig{
		Sampler: p.Sampler,
	}
}

func (p *noopProvider) Watch() <-chan InstrumentationConfig {
	c := make(chan InstrumentationConfig)
	close(c)
	return c
}

func (p *noopProvider) Shutdown(_ context.Context) error {
	return nil
}

func convertConfigProvider(cp ConfigProvider) instrumentation.ConfigProvider {
	return &converter{ConfigProvider: cp}
}

type converter struct {
	ConfigProvider

	ch     chan instrumentation.Config
	chOnce sync.Once
}

func (c *converter) InitialConfig(ctx context.Context) instrumentation.Config {
	return c.instrumentationConfig(c.ConfigProvider.InitialConfig(ctx))
}

func (c *converter) Watch() <-chan instrumentation.Config {
	c.chOnce.Do(func() {
		c.ch = make(chan instrumentation.Config)
		inCh := c.ConfigProvider.Watch()
		go func() {
			for in := range inCh {
				c.ch <- c.instrumentationConfig(in)
			}
			close(c.ch)
		}()
	})
	return c.ch
}

func (c *converter) instrumentationConfig(ic InstrumentationConfig) instrumentation.Config {
	var out instrumentation.Config

	out.DefaultTracesDisabled = ic.DefaultTracesDisabled
	if n := len(ic.InstrumentationLibraryConfigs); n > 0 {
		out.InstrumentationLibraryConfigs = make(
			map[instrumentation.LibraryID]instrumentation.Library,
			len(ic.InstrumentationLibraryConfigs),
		)
		for k, v := range ic.InstrumentationLibraryConfigs {
			id := instrumentation.LibraryID{
				InstrumentedPkg: k.InstrumentedPkg,
				SpanKind:        k.SpanKind,
			}
			out.InstrumentationLibraryConfigs[id] = instrumentation.Library{
				TracesEnabled: v.TracesEnabled,
			}
		}
	}
	out.SamplingConfig, _ = convertSamplerToConfig(ic.Sampler)

	return out
}
