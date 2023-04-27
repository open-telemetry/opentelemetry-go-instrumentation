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

package instrumentors

import (
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
)

// Instrumentor provides instrumentation for a Go package.
type Instrumentor interface {
	// LibraryName returns the package name being instrumented.
	LibraryName() string

	// FuncNames returns the fully-qualified function names that are
	// instrumented.
	FuncNames() []string

	// Load loads all instrumentation offsets.
	Load(ctx *context.InstrumentorContext) error

	// Run runs the events processing loop.
	Run(eventsChan chan<- *events.Event)

	// Close stops the Instrumentor.
	Close()
}
