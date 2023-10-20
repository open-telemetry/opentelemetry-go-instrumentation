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

	"github.com/go-logr/logr"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors/allocator"
	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/database/sql"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/github.com/gin-gonic/gin"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc"
	grpcServer "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc/server"
	httpClient "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/client"
	httpServer "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/server"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Error message returned when unable to find all instrumentation functions.
var errNotAllFuncsFound = fmt.Errorf("not all functions found for instrumentation")

// Manager handles the management of [Instrumentor] instances.
type Manager struct {
	logger         logr.Logger
	instrumentors  map[string]Instrumentor
	done           chan bool
	incomingEvents chan *events.Event
	otelController *opentelemetry.Controller
	allocator      *allocator.Allocator
}

// NewManager returns a new [Manager].
func NewManager(logger logr.Logger, otelController *opentelemetry.Controller) (*Manager, error) {
	logger = logger.WithName("Manager")
	m := &Manager{
		logger:         logger,
		instrumentors:  make(map[string]Instrumentor),
		done:           make(chan bool, 1),
		incomingEvents: make(chan *events.Event),
		otelController: otelController,
		allocator:      allocator.New(logger),
	}

	err := m.registerInstrumentors()
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
		funcsFound := 0
		for _, instF := range inst.FuncNames() {
			if _, exists := existingFuncMap[instF]; exists {
				funcsFound++
			}
		}

		if funcsFound != len(inst.FuncNames()) {
			if funcsFound > 0 {
				m.logger.Error(errNotAllFuncsFound, "some of expected functions not found - check instrumented functions", "instrumentation_name", name, "funcs_found", funcsFound, "funcs_expected", len(inst.FuncNames()))
			}
			delete(m.instrumentors, name)
		}
	}
}

func (m *Manager) registerInstrumentors() error {
	insts := []Instrumentor{
		grpc.New(m.logger),
		grpcServer.New(m.logger),
		httpServer.New(m.logger),
		httpClient.New(m.logger),
		gin.New(m.logger),
		dbSql.New(m.logger),
	}

	for _, i := range insts {
		err := m.registerInstrumentor(i)
		if err != nil {
			return err
		}
	}

	return nil
}
