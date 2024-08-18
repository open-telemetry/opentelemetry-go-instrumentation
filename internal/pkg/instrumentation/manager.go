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

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/config"
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
	cp              config.Provider
	exe             *link.Executable
	td              *process.TargetDetails
	runningProbesWG sync.WaitGroup
	eventCh         chan *probe.Event
	currentConfig   config.InstrumentationConfig
}

// NewManager returns a new [Manager].
func NewManager(logger logr.Logger, otelController *opentelemetry.Controller, globalImpl bool, loadIndicator chan struct{}, cp config.Provider) (*Manager, error) {
	logger = logger.WithName("Manager")
	m := &Manager{
		logger:          logger,
		probes:          make(map[probe.ID]probe.Probe),
		otelController:  otelController,
		globalImpl:      globalImpl,
		loadedIndicator: loadIndicator,
		cp:              cp,
		eventCh:         make(chan *probe.Event),
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

func getProbeConfig(id probe.ID, c config.InstrumentationConfig) (config.InstrumentationLibrary, bool) {
	libKindID := config.InstrumentationLibraryID{
		InstrumentedPkg: id.InstrumentedPkg,
		SpanKind:        id.SpanKind,
	}

	if lib, ok := c.InstrumentationLibraryConfigs[libKindID]; ok {
		return lib, true
	}

	libID := config.InstrumentationLibraryID{
		InstrumentedPkg: id.InstrumentedPkg,
		SpanKind:        trace.SpanKindUnspecified,
	}

	if lib, ok := c.InstrumentationLibraryConfigs[libID]; ok {
		return lib, true
	}

	return config.InstrumentationLibrary{}, false
}

func isProbeEnabled(id probe.ID, c config.InstrumentationConfig) bool {
	if pc, ok := getProbeConfig(id, c); ok {
		return pc.Enabled
	}
	return !c.DefaultDisabled
}

func (m *Manager) applyConfig(c config.InstrumentationConfig) error {
	if m.td == nil {
		return errors.New("failed to apply config: target details not set")
	}
	if m.exe == nil {
		return errors.New("failed to apply config: executable not set")
	}

	var err error

	for id, p := range m.probes {
		currentlyEnabled := isProbeEnabled(id, m.currentConfig)
		newEnabled := isProbeEnabled(id, c)

		if currentlyEnabled && !newEnabled {
			m.logger.Info("Disabling probe", "id", id)
			err = errors.Join(err, p.Close())
			continue
		}

		if !currentlyEnabled && newEnabled {
			m.logger.Info("Enabling probe", "id", id)
			err = errors.Join(err, p.Load(m.exe, m.td))
			if err == nil {
				m.runProbe(p)
			}
			continue
		}
	}

	return nil
}

func (m *Manager) runProbe(p probe.Probe) {
	m.runningProbesWG.Add(1)
	go func(ap probe.Probe) {
		defer m.runningProbesWG.Done()
		ap.Run(m.eventCh)
	}(p)
}

func (m *Manager) ConfigLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c, ok := <-m.cp.Watch():
			if !ok {
				m.logger.Info("Configuration provider closed, configuration updates will no longer be received")
				return
			}
			err := m.applyConfig(c)
			if err != nil {
				m.logger.Error(err, "Failed to apply config")
				continue
			}
			m.currentConfig = c
		}
	}
}

// Run runs the event processing loop for all managed probes.
func (m *Manager) Run(ctx context.Context, target *process.TargetDetails) error {
	if len(m.probes) == 0 {
		return errors.New("no instrumentation for target process")
	}
	if m.cp == nil {
		return errors.New("no config provider set")
	}

	m.currentConfig = m.cp.InitialConfig(ctx)

	err := m.load(target)
	if err != nil {
		return err
	}

	for id, p := range m.probes {
		if isProbeEnabled(id, m.currentConfig) {
			m.runProbe(p)
		}
	}

	if m.loadedIndicator != nil {
		close(m.loadedIndicator)
	}

	go m.ConfigLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.V(1).Info("Shutting down all probes")
			err := m.cleanup(target)

			// Wait for all probes to stop before closing the chan they send on.
			m.runningProbesWG.Wait()
			close(m.eventCh)

			return errors.Join(err, ctx.Err())
		case e := <-m.eventCh:
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
	m.exe = exe

	if m.td == nil {
		m.td = target
	}

	if err := m.mount(target); err != nil {
		return err
	}

	// Load probes
	for name, i := range m.probes {
		if isProbeEnabled(name, m.currentConfig) {
			m.logger.V(0).Info("loading probe", "name", name)
			err := i.Load(exe, target)
			if err != nil {
				m.logger.Error(err, "error while loading probes, cleaning up", "name", name)
				return errors.Join(err, m.cleanup(target))
			}
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
	err = errors.Join(err, m.cp.Shutdown(context.Background()))
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
