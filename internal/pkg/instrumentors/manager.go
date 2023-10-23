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
	"context"
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/database/sql"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/github.com/gin-gonic/gin"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc"
	grpcServer "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc/server"
	httpClient "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/client"
	httpServer "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/server"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"
	iCtx "go.opentelemetry.io/auto/internal/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Error message returned when unable to find all instrumentation functions.
var errNotAllFuncsFound = fmt.Errorf("not all functions found for instrumentation")

// Manager handles the management of [Instrumentor] instances.
type Manager struct {
	instrumentors  map[string]Instrumentor
	done           chan bool
	incomingEvents chan *events.Event
	otelController *opentelemetry.Controller
}

// NewManager returns a new [Manager].
func NewManager(otelController *opentelemetry.Controller) (*Manager, error) {
	m := &Manager{
		instrumentors:  make(map[string]Instrumentor),
		done:           make(chan bool, 1),
		incomingEvents: make(chan *events.Event),
		otelController: otelController,
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
		funcsFound := 0
		for _, instF := range inst.FuncNames() {
			if _, exists := existingFuncMap[instF]; exists {
				funcsFound++
			}
		}

		if funcsFound != len(inst.FuncNames()) {
			if funcsFound > 0 {
				log.Logger.Error(errNotAllFuncsFound, "some of expected functions not found - check instrumented functions", "instrumentation_name", name, "funcs_found", funcsFound, "funcs_expected", len(inst.FuncNames()))
			}
			delete(m.instrumentors, name)
		}
	}
}

// Run runs the event processing loop for all managed Instrumentors.
func (m *Manager) Run(ctx context.Context, target *process.TargetDetails) error {
	if len(m.instrumentors) == 0 {
		log.Logger.V(0).Info("there are no available instrumentations for target process")
		return nil
	}

	err := m.load(target)
	if err != nil {
		return err
	}

	for _, i := range m.instrumentors {
		go i.Run(m.incomingEvents)
	}

	for {
		select {
		case <-ctx.Done():
			m.Close()
			m.cleanup(target)
			return ctx.Err()
		case <-m.done:
			log.Logger.V(0).Info("shutting down all instrumentors due to signal")
			m.cleanup(target)
			return nil
		case e := <-m.incomingEvents:
			m.otelController.Trace(e)
		}
	}
}

func (m *Manager) load(target *process.TargetDetails) error {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return err
	}

	injector, err := inject.New(target)
	if err != nil {
		return err
	}

	exe, err := link.OpenExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}
	ctx := &iCtx.InstrumentorContext{
		TargetDetails: target,
		Executable:    exe,
		Injector:      injector,
	}

	if err := m.mount(target); err != nil {
		return err
	}

	// Load instrumentors
	for name, i := range m.instrumentors {
		log.Logger.V(0).Info("loading instrumentor", "name", name)
		err := i.Load(ctx)
		if err != nil {
			log.Logger.Error(err, "error while loading instrumentors, cleaning up", "name", name)
			m.cleanup(target)
			return err
		}
	}

	log.Logger.V(0).Info("loaded instrumentors to memory", "total_instrumentors", len(m.instrumentors))
	return nil
}

func (m *Manager) mount(target *process.TargetDetails) error {
	if target.AllocationDetails != nil {
		log.Logger.Info("Mounting bpffs", target.AllocationDetails)
	} else {
		log.Logger.Info("Mounting bpffs")
	}
	return bpffs.Mount(target)
}

func (m *Manager) cleanup(target *process.TargetDetails) {
	close(m.incomingEvents)
	for _, i := range m.instrumentors {
		i.Close()
	}

	log.Logger.V(0).Info("Cleaning bpffs")
	err := bpffs.Cleanup(target)
	if err != nil {
		log.Logger.Error(err, "Failed to clean bpffs")
	}
}

// Close closes m.
func (m *Manager) Close() {
	m.done <- true
}

func registerInstrumentors(m *Manager) error {
	insts := []Instrumentor{
		grpc.New(),
		grpcServer.New(),
		httpServer.New(),
		httpClient.New(),
		gin.New(),
		dbSql.New(),
	}

	for _, i := range insts {
		err := m.registerInstrumentor(i)
		if err != nil {
			return err
		}
	}

	return nil
}
