// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

const help = `Usage of %s:
  -global-impl
    	Record telemetry from the OpenTelemetry default global implementation
  -log-level string
    	logging level ("debug", "info", "warn", "error")

Runs the OpenTelemetry auto-instrumentation for Go applications using eBPF.

Environment variable configuration:

	- OTEL_GO_AUTO_TARGET_EXE: sets the target binary
	- OTEL_LOG_LEVEL: sets the log level (flag takes precedence)
	- OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES): sets the service name
	- OTEL_TRACES_EXPORTER: sets the trace exporter

The OTEL_TRACES_EXPORTER environment variable value is resolved using the
autoexport (go.opentelemetry.io/contrib/exporters/autoexport) package. See that
package's documentation for information on supported values and registration of
custom exporters.
`

// envLogLevelKey is the key for the environment variable value containing the
// log level.
const envLogLevelKey = "OTEL_LOG_LEVEL"

func usage() {
	program := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, help, program)
}

func newLogger(lvlStr string) *slog.Logger {
	levelVar := new(slog.LevelVar) // Default value of info.
	opts := &slog.HandlerOptions{AddSource: true, Level: levelVar}
	h := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(h)

	if lvlStr == "" {
		lvlStr = os.Getenv(envLogLevelKey)
	}

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

func main() {
	var globalImpl bool
	var logLevel string

	flag.BoolVar(&globalImpl, "global-impl", false, "Record telemetry from the OpenTelemetry default global implementation")
	flag.StringVar(&logLevel, "log-level", "", `logging level ("debug", "info", "warn", "error")`)

	flag.Usage = usage
	flag.Parse()

	logger := newLogger(logLevel)

	// Trap Ctrl+C and SIGTERM and call cancel on the context.
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(ch)
		cancel()
	}()
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()

	logger.Info(
		"building OpenTelemetry Go instrumentation ...",
		"globalImpl", globalImpl,
		"version", newVersion(),
	)

	instOptions := []auto.InstrumentationOption{
		auto.WithEnv(),
		auto.WithLogger(logger),
	}
	if globalImpl {
		instOptions = append(instOptions, auto.WithGlobal())
	}

	inst, err := auto.NewInstrumentation(ctx, instOptions...)
	if err != nil {
		logger.Error("failed to create instrumentation", "error", err)
		return
	}

	err = inst.Load(ctx)
	if err != nil {
		logger.Error("failed to load instrumentation", "error", err)
		return
	}

	logger.Info("instrumentation loaded successfully, starting...")

	if err = inst.Run(ctx); err != nil && !errors.Is(err, process.ErrInterrupted) {
		logger.Error("instrumentation crashed", "error", err)
	}
}
