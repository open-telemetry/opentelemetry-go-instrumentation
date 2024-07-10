// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

const help = `
OpenTelemetry auto-instrumentation for Go applications using eBPF

Environment variable configuration:

	- OTEL_GO_AUTO_TARGET_EXE: sets the target binary
	- OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES): sets the service name
	- OTEL_TRACES_EXPORTER: sets the trace exporter

The OTEL_TRACES_EXPORTER environment variable value is resolved using the
autoexport (go.opentelemetry.io/contrib/exporters/autoexport) package. See that
package's documentation for information on supported values and registration of
custom exporters.
`

func usage() {
	fmt.Fprintf(os.Stderr, "%s", help)
}

func newLogger() logr.Logger {
	zapLog, err := zap.NewProduction()

	var logger logr.Logger
	if err != nil {
		// Fallback to stdr logger.
		logger = stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	} else {
		logger = zapr.NewLogger(zapLog)
	}

	return logger
}

func main() {
	var globalImpl bool
	var logLevel string

	flag.BoolVar(&globalImpl, "global-impl", false, "Record telemetry from the OpenTelemetry default global implementation")
	flag.StringVar(&logLevel, "log-level", "", "Define log visibility level, default is `info`")

	flag.Usage = usage
	flag.Parse()

	logger := newLogger().WithName("go.opentelemetry.io/auto")

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

	logger.Info("building OpenTelemetry Go instrumentation ...", "globalImpl", globalImpl)

	loadedIndicator := make(chan struct{})
	instOptions := []auto.InstrumentationOption{auto.WithEnv(), auto.WithLoadedIndicator(loadedIndicator)}
	if globalImpl {
		instOptions = append(instOptions, auto.WithGlobal())
	}

	if logLevel != "" {
		level, err := auto.ParseLogLevel(logLevel)
		if err != nil {
			logger.Error(err, "failed to parse log level")
			return
		}

		instOptions = append(instOptions, auto.WithLogLevel(level))
	}

	inst, err := auto.NewInstrumentation(ctx, instOptions...)
	if err != nil {
		logger.Error(err, "failed to create instrumentation")
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-loadedIndicator:
			logger.Info("instrumentation loaded successfully")
		}
	}()

	logger.Info("starting instrumentation...")
	if err = inst.Run(ctx); err != nil && !errors.Is(err, process.ErrInterrupted) {
		logger.Error(err, "instrumentation crashed")
	}
}
