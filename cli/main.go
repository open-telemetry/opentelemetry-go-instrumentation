// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	flag.BoolVar(&globalImpl, "global-impl", false, "Record telemetry from the OpenTelemetry default global implementation")
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

	instOptions := []auto.InstrumentationOption{auto.WithEnv()}
	if globalImpl {
		instOptions = append(instOptions, auto.WithGlobal())
	}

	inst, err := auto.NewInstrumentation(ctx, instOptions...)
	if err != nil {
		logger.Error(err, "failed to create instrumentation")
		return
	}

	logger.Info("starting instrumentation...")
	if err = inst.Run(ctx); err != nil && !errors.Is(err, process.ErrInterrupted) {
		logger.Error(err, "instrumentation crashed")
	}
}
