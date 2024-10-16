// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/connector"
	forwardconnector "go.opentelemetry.io/collector/connector/forwardconnector"
	"go.opentelemetry.io/collector/exporter"
	otlphttpexporter "go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	batchprocessor "go.opentelemetry.io/collector/processor/batchprocessor"
	memorylimiterprocessor "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
)

type Handler interface {
	Shutdown(context.Context) error
}

type TraceHandler interface {
	Handler

	HandleScopeSpans(context.Context, ptrace.ScopeSpans) error
}

func defaultTraceHandler(ctx context.Context, l *slog.Logger) (TraceHandler, error) {
	r := NewReceiver(l)

	info := component.BuildInfo{
		Command:     "auto",
		Description: "OpenTelemetry Collector for Go Auto-Instrumentation",
		Version:     Version(),
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components(r),
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					confmap.NewProviderFactory(newProvider),
				},
			},
		},
	}

	c, err := otelcol.NewCollector(set)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := c.Run(ctx); err != nil {
			l.Error("collector server failed to run", "error", err)
		}
	}()

	return r, nil
}

func components(r *Receiver) func() (otelcol.Factories, error) {
	return func() (otelcol.Factories, error) {
		var (
			f   otelcol.Factories
			err error
		)

		f.Receivers, err = receiver.MakeFactoryMap(r.Factory())
		if err != nil {
			return otelcol.Factories{}, err
		}
		f.ReceiverModules = map[component.Type]string{
			r.Factory().Type(): "go.opentelemetry.io/auto " + Version(),
		}

		f.Exporters, err = exporter.MakeFactoryMap(otlphttpexporter.NewFactory())
		if err != nil {
			return otelcol.Factories{}, err
		}
		f.ExporterModules = map[component.Type]string{
			otlphttpexporter.NewFactory().Type(): "go.opentelemetry.io/collector/exporter/otlphttpexporter v0.110.0",
		}

		f.Processors, err = processor.MakeFactoryMap(
			batchprocessor.NewFactory(),
			memorylimiterprocessor.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}
		f.ProcessorModules = make(map[component.Type]string, len(f.Processors))
		f.ProcessorModules[batchprocessor.NewFactory().Type()] = "go.opentelemetry.io/collector/processor/batchprocessor v0.110.0"
		f.ProcessorModules[memorylimiterprocessor.NewFactory().Type()] = "go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.110.0"

		f.Connectors, err = connector.MakeFactoryMap(
			forwardconnector.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}
		f.ConnectorModules = make(map[component.Type]string, len(f.Connectors))
		f.ConnectorModules[forwardconnector.NewFactory().Type()] = "go.opentelemetry.io/collector/connector/forwardconnector v0.110.0"

		return f, nil
	}
}

const providerSchema = "static"

type provider struct{}

func newProvider(confmap.ProviderSettings) confmap.Provider {
	return new(provider)
}

func (p *provider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, providerSchema+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, providerSchema)
	}

	return confmap.NewRetrieved(map[string]any{
		"receivers": map[string]any{
			"auto": nil,
		},
		"processors": map[string]any{
			"batch": nil,
			"memory_limiter": map[string]any{
				"check_interval":  "5s",
				"limit_mib":       4000,
				"spike_limit_mib": 500,
			},
		},
		"exporters": map[string]any{
			"otlphttp": nil,
		},
		"service": map[string]any{
			"pipelines": map[string]any{
				"traces": map[string]any{
					"receivers":  []string{"otlp"},
					"processors": []string{"batch", "memory_limiter"},
					"exporters":  []string{"otlphttp"},
				},
			},
		},
	})
}

func (p *provider) Scheme() string {
	return providerSchema
}

func (p *provider) Shutdown(context.Context) error {
	return nil
}
