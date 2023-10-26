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

package opentelemetry

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func DefaultTraceExporter(ctx context.Context, version string) (trace.SpanExporter, error) {
	// Controller-local reference to the auto-instrumentation release
	// version. Start of this auto-instrumentation's exporter User-Agent
	// header, e.g. ""OTel-Go-Auto-Instrumentation/1.2.3".
	baseUserAgent := fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", version)
	autoinstUserAgent := fmt.Sprintf("%s %s", baseUserAgent, runtimeInfo)
	otlpTraceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithDialOption(grpc.WithUserAgent(autoinstUserAgent)),
	)

	return otlptrace.New(ctx, otlpTraceClient)
}
