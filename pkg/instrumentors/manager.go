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

	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/allocator"
	gorillaMux "github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/github.com/gorilla/mux"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc"
	grpcServer "github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc/server"
	httpServer "github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/net/http/server"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/log"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/opentelemetry"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/process"
)

type instrumentorsManager struct {
	instrumentors  map[string]Instrumentor
	done           chan bool
	incomingEvents chan *events.Event
	otelController *opentelemetry.Controller
	allocator      *allocator.Allocator
}

func NewManager(otelController *opentelemetry.Controller) (*instrumentorsManager, error) {
	m := &instrumentorsManager{
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

func (m *instrumentorsManager) registerInstrumentor(instrumentor Instrumentor) error {
	if _, exists := m.instrumentors[instrumentor.LibraryName()]; exists {
		return fmt.Errorf("library %s registered twice, aborting", instrumentor.LibraryName())
	}

	m.instrumentors[instrumentor.LibraryName()] = instrumentor
	return nil
}

func (m *instrumentorsManager) GetRelevantFuncs() map[string]interface{} {
	funcsMap := make(map[string]interface{})
	for _, i := range m.instrumentors {
		for _, f := range i.FuncNames() {
			funcsMap[f] = nil
		}
	}

	return funcsMap
}

func (m *instrumentorsManager) FilterUnusedInstrumentors(target *process.TargetDetails) {
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

func registerInstrumentors(m *instrumentorsManager) error {
	insts := []Instrumentor{
		grpc.New(),
		grpcServer.New(),
		httpServer.New(),
		gorillaMux.New(),
	}

	for _, i := range insts {
		err := m.registerInstrumentor(i)
		if err != nil {
			return err
		}
	}

	return nil
}
