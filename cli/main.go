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

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"google.golang.org/grpc"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/orchestrator"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

var (
	// Start of this auto-instrumentation's exporter User-Agent header, e.g. ""OTel-Go-Auto-Instrumentation/1.2.3".
	baseUserAgent = fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", auto.Version())
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
	if err = r.Run(); err != nil {
		log.Logger.Error(err, "running orchestrator")
	}
}

