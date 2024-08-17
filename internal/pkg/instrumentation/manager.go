// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

// Function variables overridden in testing.
var (
	openExecutable      = link.OpenExecutable
	rlimitRemoveMemlock = rlimit.RemoveMemlock
	bpffsMount          = bpffs.Mount
	bpffsCleanup        = bpffs.Cleanup
)

// Manager handles the management of [probe.Probe] instances.
type Manager struct {
	logger          logr.Logger
	probes          map[probe.ID]probe.Probe
	otelController  *opentelemetry.Controller
	globalImpl      bool
	loadedIndicator chan struct{}
}

// NewManager returns a new [Manager].
func NewManager(logger logr.Logger, otelController *opentelemetry.Controller, globalImpl bool, loadIndicator chan struct{}) (*Manager, error) {
	logger = logger.WithName("Manager")
	m := &Manager{
		logger:          logger,
		probes:          make(map[probe.ID]probe.Probe),
		otelController:  otelController,
		globalImpl:      globalImpl,
		loadedIndicator: loadIndicator,
	}

	err := m.registerProbes()
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) validateProbeDependents(id probe.ID, symbols []probe.FunctionSymbol) error {
	// Validate that dependent probes point to real standalone probes.
	funcsMap := make(map[string]interface{})
	for _, s := range symbols {
		funcsMap[s.Symbol] = nil
	}

	for _, s := range symbols {
		for _, d := range s.DependsOn {
			if _, exists := funcsMap[d]; !exists {
				return fmt.Errorf("library %s has declared a dependent function %s for probe %s which does not exist, aborting", id, d, s.Symbol)
			}
		}
	}

	return nil
}

func (m *Manager) registerProbe(p probe.Probe) error {
	id := p.Manifest().Id
	if _, exists := m.probes[id]; exists {
		return fmt.Errorf("library %s registered twice, aborting", id)
	}

	if err := m.validateProbeDependents(id, p.Manifest().Symbols); err != nil {
		return err
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
			m.logger.V(1).Info("no functions found for probe, removing", "name", name)
			delete(m.probes, name)
		}
	}
}

// Run runs the event processing loop for all managed probes.
func (m *Manager) Run(ctx context.Context, target *process.TargetDetails) error {
	if len(m.probes) == 0 {
		return errors.New("no instrumentation for target process")
	}

	err := m.load(target)
	if err != nil {
		return err
	}

	eventCh := make(chan *probe.Event)
	var wg sync.WaitGroup
	for _, i := range m.probes {
		wg.Add(1)
		go func(p probe.Probe) {
			defer wg.Done()
			p.Run(eventCh)
		}(i)
	}

	if m.loadedIndicator != nil {
		close(m.loadedIndicator)
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.V(1).Info("Shutting down all probes")
			err := m.cleanup(target)

			// Wait for all probes to stop before closing the chan they send on.
			wg.Wait()
			close(eventCh)

			return errors.Join(err, ctx.Err())
		case e := <-eventCh:
			m.otelController.Trace(e)
		}
	}
}

func (m *Manager) load(target *process.TargetDetails) error {
	// Remove resource limits for kernels <5.11.
	if err := rlimitRemoveMemlock(); err != nil {
		return err
	}

	exe, err := openExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}

	if err := m.mount(target); err != nil {
		return err
	}

	// Load probes
	for name, i := range m.probes {
		m.logger.V(0).Info("loading probe", "name", name)
		err := i.Load(exe, target)
		if err != nil {
			m.logger.Error(err, "error while loading probes, cleaning up", "name", name)
			return errors.Join(err, m.cleanup(target))
		}
	}

	m.logger.V(1).Info("loaded probes to memory", "total_probes", len(m.probes))
	return nil
}

func (m *Manager) mount(target *process.TargetDetails) error {
	if target.AllocationDetails != nil {
		m.logger.V(1).Info("Mounting bpffs", "allocations_details", target.AllocationDetails)
	} else {
		m.logger.V(1).Info("Mounting bpffs")
	}
	return bpffsMount(target)
}

func (m *Manager) cleanup(target *process.TargetDetails) error {
	var err error
	for _, i := range m.probes {
		err = errors.Join(err, i.Close())
	}

	m.logger.V(1).Info("Cleaning bpffs")
	return errors.Join(err, bpffsCleanup(target))
}

//nolint:revive // ignoring linter complaint about control flag
func availableProbes(l logr.Logger, withTraceGlobal bool) []probe.Probe {
	insts := []probe.Probe{
		grpcClient.New(l),
		grpcServer.New(l),
		httpServer.New(l),
		httpClient.New(l),
		dbSql.New(l),
		kafkaProducer.New(l),
		kafkaConsumer.New(l),
	}

	if withTraceGlobal {
		insts = append(insts, otelTraceGlobal.New(l))
	}

	return insts
}

func (m *Manager) registerProbes() error {
	insts := availableProbes(m.logger, m.globalImpl)

	for _, i := range insts {
		err := m.registerProbe(i)
		if err != nil {
			return err
		}
	}

	return nil
}
