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
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/orchestrator"
	"go.opentelemetry.io/auto/pkg/process"
)

var (
	// Controller-local reference to the auto-instrumentation release version.
	releaseVersion = auto.Version()
	// Start of this auto-instrumentation's exporter User-Agent header, e.g. ""OTel-Go-Auto-Instrumentation/1.2.3".
	baseUserAgent = fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", releaseVersion)
	// Information about the runtime environment for inclusion in User-Agent, e.g. "go/1.18.2 (linux/amd64)".
	runtimeInfo = fmt.Sprintf(
		"%s (%s/%s)",
		strings.Replace(runtime.Version(), "go", "go/", 1),
		runtime.GOOS,
		runtime.GOARCH,
	)
	// Combined User-Agent identifying this auto-instrumentation and its runtime environment, see RFC7231 for format considerations.
	autoinstUserAgent = fmt.Sprintf("%s %s", baseUserAgent, runtimeInfo)
)

func main() {
	err := log.Init()
	if err != nil {
		fmt.Printf("could not init logger: %s\n", err)
		os.Exit(1)
	}

	log.Logger.V(0).Info("starting Go OpenTelemetry Agent ...")
	ctx := contextWithSigterm(context.Background())
	log.Logger.V(0).Info("Establishing connection to OTLP receiver ...")
	otlpTraceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithDialOption(grpc.WithUserAgent(autoinstUserAgent)),
	)
	traceExporter, err := otlptrace.New(ctx, otlpTraceClient)
	if err != nil {
		log.Logger.Error(err, "unable to connect to OTLP endpoint")
		return
	}
	targetArgs := process.ParseTargetArgs()
	if targetArgs != nil {
		if err := targetArgs.Validate(); err != nil {
			log.Logger.Error(err, "invalid target args")
			return
		}
	}
	r, err := orchestrator.New(ctx, targetArgs, traceExporter)
	if err != nil {
		log.Logger.V(0).Error(err, "creating orchestrator")
	}
	if err = r.Run(); err != nil {
		log.Logger.Error(err, "running orchestrator")
	}
}

func contextWithSigterm(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer close(ch)
		defer signal.Stop(ch)

		select {
		case <-parent.Done(): // if parent is cancelled, return
			return
		case <-ch: // if SIGTERM is received, cancel this context
			cancel()
		}
	}()

	return ctx
}
