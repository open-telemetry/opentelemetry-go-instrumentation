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

package instrumentation

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/go-logr/logr"

	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/database/sql"
	kafkaConsumer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/consumer"
	kafkaProducer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/producer"
	otelTraceGlobal "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/otel/traceglobal"
	grpcClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client"
	grpcServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/server"
	httpClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/client"
	httpServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/server"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Manager handles the management of [probe.Probe] instances.
type Manager struct {
	logger         logr.Logger
	probes         map[probe.ID]probe.Probe
	done           chan bool
	incomingEvents chan *probe.Event
	otelController *opentelemetry.Controller
	globalImpl     bool
	wg             sync.WaitGroup
	closingErrors  chan error
}

// NewManager returns a new [Manager].
func NewManager(logger logr.Logger, otelController *opentelemetry.Controller, globalImpl bool) (*Manager, error) {
	logger = logger.WithName("Manager")
	m := &Manager{
		logger:         logger,
		probes:         make(map[probe.ID]probe.Probe),
		done:           make(chan bool, 1),
		incomingEvents: make(chan *probe.Event),
		otelController: otelController,
		globalImpl:     globalImpl,
		closingErrors:  make(chan error, 1),
	}

	err := m.registerProbes()
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) registerProbe(p probe.Probe) error {
	id := p.Manifest().Id
	if _, exists := m.probes[id]; exists {
		return fmt.Errorf("library %s registered twice, aborting", id)
	}

	m.probes[id] = p
	return nil
}

// GetRelevantFuncs returns the instrumented functions for all managed probes.
func (m *Manager) GetRelevantFuncs() map[string]interface{} {
	funcsMap := make(map[string]interface{})
	for _, i := range m.probes {
		for _, s := range i.Manifest().Symbols {
			funcsMap[s.Symbol] = nil
		}
	}

	return funcsMap
}

// FilterUnusedProbes filterers probes whose functions are already instrumented
// out of the Manager.
func (m *Manager) FilterUnusedProbes(target *process.TargetDetails) {
	existingFuncMap := make(map[string]interface{})
	for _, f := range target.Functions {
		existingFuncMap[f.Name] = nil
	}

	for name, inst := range m.probes {
		funcsFound := false
		for _, s := range inst.Manifest().Symbols {
			if len(s.DependsOn) == 0 {
				if _, exists := existingFuncMap[s.Symbol]; exists {
					funcsFound = true
					break
				}
			}
		}

		if !funcsFound {
			m.logger.Info("no functions found for probe, removing", "name", name)
			delete(m.probes, name)
		}
	}
}

// Run runs the event processing loop for all managed probes.
func (m *Manager) Run(ctx context.Context, target *process.TargetDetails) error {
	if len(m.probes) == 0 {
		err := errors.New("no instrumentation for target process")
		close(m.closingErrors)
		return err
	}

	err := m.load(target)
	if err != nil {
		close(m.closingErrors)
		return err
	}

	m.wg.Add(len(m.probes))
	for _, i := range m.probes {
		go func(p probe.Probe) {
			defer m.wg.Done()
			p.Run(m.incomingEvents)
		}(i)
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("shutting down all probes due to context cancellation")
			err := m.cleanup(target)
			err = errors.Join(err, ctx.Err())
			m.closingErrors <- err
			return nil
		case <-m.done:
			m.logger.Info("shutting down all probes due to signal")
			err := m.cleanup(target)
			m.closingErrors <- err
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

	exe, err := link.OpenExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}

	if err := m.mount(target); err != nil {
		return err
	}

	// Load probes
	for name, i := range m.probes {
		m.logger.Info("loading probe", "name", name)
		err := i.Load(exe, target)
		if err != nil {
			m.logger.Error(err, "error while loading probes, cleaning up", "name", name)
			return errors.Join(err, m.cleanup(target))
		}
	}

	m.logger.Info("loaded probes to memory", "total_probes", len(m.probes))
	return nil
}

func (m *Manager) mount(target *process.TargetDetails) error {
	if target.AllocationDetails != nil {
		m.logger.Info("Mounting bpffs", "allocations_details", target.AllocationDetails)
	} else {
		m.logger.Info("Mounting bpffs")
	}
	return bpffs.Mount(target)
}

func (m *Manager) cleanup(target *process.TargetDetails) error {
	var err error
	close(m.incomingEvents)
	for _, i := range m.probes {
		err = errors.Join(err, i.Close())
	}

	m.logger.Info("Cleaning bpffs")
	return errors.Join(err, bpffs.Cleanup(target))
}

// Close closes m.
func (m *Manager) Close() error {
	m.done <- true
	err := <-m.closingErrors
	m.wg.Wait()
	return err
}

func (m *Manager) registerProbes() error {
	insts := []probe.Probe{
		grpcClient.New(m.logger),
		grpcServer.New(m.logger),
		httpServer.New(m.logger),
		httpClient.New(m.logger),
		dbSql.New(m.logger),
		kafkaProducer.New(m.logger),
		kafkaConsumer.New(m.logger),
	}

	if m.globalImpl {
		insts = append(insts, otelTraceGlobal.New(m.logger))
	}

	for _, i := range insts {
		err := m.registerProbe(i)
		if err != nil {
			return err
		}
	}

	return nil
}
