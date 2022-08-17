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
	"github.com/pdelewski/autotel/rtlib"
	otel "go.opentelemetry.io/otel"
	"context"
)

func recur(__tracing_ctx context.Context, n int) {
	__child_tracing_ctx, span := otel.Tracer("recur").Start(__tracing_ctx, "recur")
	_ = __child_tracing_ctx
	defer span.End()
	if n > 0 {
		recur(__child_tracing_ctx, n-1)
	}
}

func main() {
	__child_tracing_ctx := context.TODO()
	_ = __child_tracing_ctx
	ts := rtlib.NewTracingState()
	defer rtlib.Shutdown(ts)
	otel.SetTracerProvider(ts.Tp)
	ctx := context.Background()
	__child_tracing_ctx, span := otel.Tracer("main").Start(ctx, "main")
	defer span.End()
	rtlib.AutotelEntryPoint__()
	recur(__child_tracing_ctx, 5)
}
