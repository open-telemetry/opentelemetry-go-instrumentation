// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/sagikazarmark/slog-shim"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	fileprovider "go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/connector"
	forwardconnector "go.opentelemetry.io/collector/connector/forwardconnector"
	"go.opentelemetry.io/collector/exporter"
	debugexporter "go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/receiver"
	otlpreceiver "go.opentelemetry.io/collector/receiver/otlpreceiver"
)

const (
	defaultListenAddr = ":8080"
	shutdownPath      = "/shutdown"
)

const config = `
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  debug: {}
  file/trace:
    path: %s
    rotation:

service:
  extensions: 
    - health_check
  telemetry:
    logs:
      level: "debug"
  pipelines:
    traces:
      receivers:
        - otlp
      exporters:
        - file/trace
        - debug
`

func main() {
	logLevel := flag.String("log-level", "debug", `logging level ("debug", "info", "warn", "error")`)
	listen := flag.String("addr", defaultListenAddr, `Address to listen for shutdown signal on.`)
	outPath := flag.String("out", "traces-orig.json", "Path to output generated traces")
	flag.Parse()

	logger := newLogger(*logLevel)

	logger.Debug("flags", "log-level", *logLevel, "addr", *listen, "out", *outPath)

	ctx, cancel := context.WithCancel(context.Background())
	// Trap Ctrl+C and SIGTERM and call cancel on the context.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	configYaml := fmt.Sprintf(config, *outPath)
	logger.Debug("built config", "config", configYaml)
	coll := Collector{logger: logger}
	if err := coll.Start(ctx, configYaml); err != nil {
		logger.Error("failed to start collector", "error", err)
		os.Exit(1)
	}

	*listen = getEnv("SHUTDOWN_SERVER_ADDR", ":8080")

	// Start the HTTP server for shutdown endpoint
	go startHTTPServer(*listen, cancel, logger)

	// Wait for the context to be canceled
	<-ctx.Done()

	coll.Stop()
}

func newLogger(lvlStr string) *slog.Logger {
	levelVar := new(slog.LevelVar) // Default value of info.
	opts := &slog.HandlerOptions{AddSource: true, Level: levelVar}
	h := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(h)

	if lvlStr == "" {
		return logger
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(lvlStr)); err != nil {
		logger.Error("failed to parse log level", "error", err, "log-level", lvlStr)
	} else {
		levelVar.Set(level)
	}

	return logger
}

type Collector struct {
	logger *slog.Logger

	collMu sync.Mutex
	coll   *otelcol.Collector
}

func (c *Collector) Start(ctx context.Context, configYaml string) error {
	c.collMu.Lock()
	defer c.collMu.Unlock()

	c.logger.Debug("starting collector")

	info := component.BuildInfo{
		Command:     "otel-wrapper",
		Description: "Custom OpenTelemetry Collector Wrapper",
		Version:     "0.0.1",
	}

	uri := "yaml:" + configYaml
	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs: []string{uri},
				ProviderFactories: []confmap.ProviderFactory{
					fileprovider.NewFactory(),
					yamlprovider.NewFactory(),
				},
			},
		},
	}

	// Initialize the OpenTelemetry Collector
	var err error
	c.coll, err = otelcol.NewCollector(set)
	if err != nil {
		c.coll = nil
		return err
	}

	// Start the OpenTelemetry Collector
	go func() {
		err := c.coll.Run(ctx)
		if err != nil {
			c.logger.Error("failed to run the collector", "error", err)
		}
	}()
	c.logger.Info("collector started")
	return nil
}

func (c *Collector) Stop() {
	c.logger.Info("stopping collector")
	c.collMu.Lock()
	defer c.collMu.Unlock()

	if c.coll == nil {
		return
	}

	c.coll.Shutdown()
	c.logger.Info("collector stopped")
	c.coll = nil
}

func startHTTPServer(addr string, cancel context.CancelFunc, logger *slog.Logger) {
	http.HandleFunc(shutdownPath, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("shutdown endpoint hit")
		cancel()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Shutting down collector..."))
	})

	logger.Info("starting shutdown HTTP server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("failed to start HTTP server", "error", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	factories.Extensions, err = extension.MakeFactoryMap(
		healthcheckextension.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}
	factories.ExtensionModules = make(map[component.Type]string, len(factories.Extensions))
	factories.ExtensionModules[healthcheckextension.NewFactory().Type()] = "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.115.0"

	factories.Receivers, err = receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}
	factories.ReceiverModules = make(map[component.Type]string, len(factories.Receivers))
	factories.ReceiverModules[otlpreceiver.NewFactory().Type()] = "go.opentelemetry.io/collector/receiver/otlpreceiver v0.115.0"

	factories.Exporters, err = exporter.MakeFactoryMap(
		debugexporter.NewFactory(),
		fileexporter.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}
	factories.ExporterModules = make(map[component.Type]string, len(factories.Exporters))
	factories.ExporterModules[debugexporter.NewFactory().Type()] = "go.opentelemetry.io/collector/exporter/debugexporter v0.115.0"
	factories.ExporterModules[fileexporter.NewFactory().Type()] = "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.115.0"

	factories.Connectors, err = connector.MakeFactoryMap(
		forwardconnector.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}
	factories.ConnectorModules = make(map[component.Type]string, len(factories.Connectors))
	factories.ConnectorModules[forwardconnector.NewFactory().Type()] = "go.opentelemetry.io/collector/connector/forwardconnector v0.115.0"

	return factories, nil
}
