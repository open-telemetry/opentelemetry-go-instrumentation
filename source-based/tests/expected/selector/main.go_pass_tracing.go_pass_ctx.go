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

type Driver interface {
	Foo(__tracing_ctx context.Context, i int)
}

type Impl struct {
}

func (impl Impl) Foo(__tracing_ctx context.Context, i int) {
	__child_tracing_ctx, span := otel.Tracer("Foo").Start(__tracing_ctx, "Foo")
	_ = __child_tracing_ctx
	defer span.End()
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
	a := []Driver{
		Impl{},
	}
	var d Driver
	d = Impl{}
	d.Foo(__child_tracing_ctx, 3)
	a[0].Foo(__child_tracing_ctx, 4)
}
