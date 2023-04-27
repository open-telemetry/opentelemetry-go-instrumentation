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
	"fmt"

	"go.opentelemetry.io/auto/pkg/instrumentors/allocator"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gin-gonic/gin"
	gorillaMux "go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/gorilla/mux"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc"
	grpcServer "go.opentelemetry.io/auto/pkg/instrumentors/bpf/google/golang/org/grpc/server"
	httpServer "go.opentelemetry.io/auto/pkg/instrumentors/bpf/net/http/server"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/opentelemetry"
	"go.opentelemetry.io/auto/pkg/process"
)

// Manager handles the management of [Instrumentor] instances.
type Manager struct {
	instrumentors  map[string]Instrumentor
	done           chan bool
	incomingEvents chan *events.Event
	otelController *opentelemetry.Controller
	allocator      *allocator.Allocator
}

// NewManager returns a new [Manager].
func NewManager(otelController *opentelemetry.Controller) (*Manager, error) {
	m := &Manager{
		instrumentors:  make(map[string]Instrumentor),
		done:           make(chan bool, 1),
		incomingEvents: make(chan *events.Event),
		otelController: otelController,
		allocator:      allocator.New(),
	}

	err := registerInstrumentors(m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) registerInstrumentor(instrumentor Instrumentor) error {
	if _, exists := m.instrumentors[instrumentor.LibraryName()]; exists {
		return fmt.Errorf("library %s registered twice, aborting", instrumentor.LibraryName())
	}

	m.instrumentors[instrumentor.LibraryName()] = instrumentor
	return nil
}

// GetRelevantFuncs returns the instrumented functions for all managed
// Instrumentors.
func (m *Manager) GetRelevantFuncs() map[string]interface{} {
	funcsMap := make(map[string]interface{})
	for _, i := range m.instrumentors {
		for _, f := range i.FuncNames() {
			funcsMap[f] = nil
		}
	}

	return funcsMap
}

// FilterUnusedInstrumentors filterers Instrumentors whose functions are
// already instrumented out of the Manager.
func (m *Manager) FilterUnusedInstrumentors(target *process.TargetDetails) {
	existingFuncMap := make(map[string]interface{})
	for _, f := range target.Functions {
		existingFuncMap[f.Name] = nil
	}

	for name, inst := range m.instrumentors {
		allFuncExists := true
		for _, instF := range inst.FuncNames() {
			if _, exists := existingFuncMap[instF]; !exists {
				allFuncExists = false
				break
			}
		}

		if !allFuncExists {
			log.Logger.V(1).Info("filtering unused instrumentation", "name", name)
			delete(m.instrumentors, name)
		}
	}
}

func registerInstrumentors(m *Manager) error {
	insts := []Instrumentor{
		grpc.New(),
		grpcServer.New(),
		httpServer.New(),
		gorillaMux.New(),
		gin.New(),
	}

	for _, i := range insts {
		err := m.registerInstrumentor(i)
		if err != nil {
			return err
		}
	}

	return nil
}
