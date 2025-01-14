// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"go.opentelemetry.io/otel/trace"

	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/database/sql"
	kafkaConsumer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/consumer"
	kafkaProducer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/producer"
	autosdk "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/auto/sdk"
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

type managerState int

const (
	managerStateUninitialized managerState = iota
	managerStateLoaded
	managerStateRunning
	managerStateStopped
)

// Manager handles the management of [probe.Probe] instances.
type Manager struct {
	logger          *slog.Logger
	version         string
	probes          map[probe.ID]probe.Probe
	otelController  *opentelemetry.Controller
	globalImpl      bool
	cp              ConfigProvider
	exe             *link.Executable
	td              *process.TargetDetails
	stop            context.CancelCauseFunc
	runningProbesWG sync.WaitGroup
	currentConfig   Config
	probeMu         sync.Mutex
	state           managerState
	stateMu         sync.RWMutex
}

// NewManager returns a new [Manager].
func NewManager(logger *slog.Logger, otelController *opentelemetry.Controller, globalImpl bool, cp ConfigProvider, version string) (*Manager, error) {
	m := &Manager{
		logger:         logger,
		version:        version,
		probes:         make(map[probe.ID]probe.Probe),
		otelController: otelController,
		globalImpl:     globalImpl,
		cp:             cp,
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
	id := p.Manifest().ID
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
			m.logger.Debug("no functions found for probe, removing", "name", name)
			delete(m.probes, name)
		}
	}
}

func getProbeConfig(id probe.ID, c Config) (Library, bool) {
	libKindID := LibraryID{
		InstrumentedPkg: id.InstrumentedPkg,
		SpanKind:        id.SpanKind,
	}

	if lib, ok := c.InstrumentationLibraryConfigs[libKindID]; ok {
		return lib, true
	}

	libID := LibraryID{
		InstrumentedPkg: id.InstrumentedPkg,
		SpanKind:        trace.SpanKindUnspecified,
	}

	if lib, ok := c.InstrumentationLibraryConfigs[libID]; ok {
		return lib, true
	}

	return Library{}, false
}

func isProbeEnabled(id probe.ID, c Config) bool {
	if pc, ok := getProbeConfig(id, c); ok && pc.TracesEnabled != nil {
		return *pc.TracesEnabled
	}
	return !c.DefaultTracesDisabled
}

func (m *Manager) applyConfig(c Config) error {
	if m.td == nil {
		return errors.New("failed to apply config: target details not set")
	}
	if m.exe == nil {
		return errors.New("failed to apply config: executable not set")
	}

	var err error
	m.probeMu.Lock()
	defer m.probeMu.Unlock()

	if m.state != managerStateRunning {
		return nil
	}

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
			err = errors.Join(err, p.Load(m.exe, m.td, c.SamplingConfig))
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
		ap.Run(m.otelController.Trace)
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
				m.logger.Error("Failed to apply config", "error", err)
				continue
			}
			m.currentConfig = c
		}
	}
}

func (m *Manager) Load(ctx context.Context, target *process.TargetDetails) error {
	if len(m.probes) == 0 {
		return errors.New("no instrumentation for target process")
	}
	if m.cp == nil {
		return errors.New("no config provider set")
	}
	if target == nil {
		return errors.New("target details not set - load is called on non-initialized instrumentation")
	}
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if m.state == managerStateRunning {
		return errors.New("manager is already running, load is not allowed")
	}

	m.currentConfig = m.cp.InitialConfig(ctx)
	err := m.loadProbes(target)
	if err != nil {
		return err
	}

	m.td = target
	m.state = managerStateLoaded

	return nil
}

func (m *Manager) runProbes(ctx context.Context) (context.Context, error) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if m.state != managerStateLoaded {
		return nil, errors.New("manager is not loaded, call Load before Run")
	}

	for id, p := range m.probes {
		if isProbeEnabled(id, m.currentConfig) {
			m.runProbe(p)
		}
	}

	ctx, stop := context.WithCancelCause(ctx)
	m.stop = stop
	m.state = managerStateRunning
	return ctx, nil
}

// Run runs the event processing loop for all managed probes.
func (m *Manager) Run(ctx context.Context) error {
	ctx, err := m.runProbes(ctx)
	if err != nil {
		return err
	}

	go m.ConfigLoop(ctx)

	done := make(chan error, 1)
	go func() {
		defer close(done)
		<-ctx.Done()

		err := m.Stop()
		if e := context.Cause(ctx); !errors.Is(e, errStop) {
			err = errors.Join(err, e)
		}
		done <- err
	}()

	return <-done
}

var errStop = errors.New("stopped called")

// Stop stops all probes and cleans up all the resources associated with them.
func (m *Manager) Stop() error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	currentState := m.state
	if currentState == managerStateUninitialized || currentState == managerStateStopped {
		return nil
	}

	if currentState == managerStateRunning {
		m.stop(errStop)
	}

	m.probeMu.Lock()
	defer m.probeMu.Unlock()

	m.logger.Debug("Shutting down all probes")
	err := m.cleanup(m.td)

	// Wait for all probes to stop.
	m.runningProbesWG.Wait()

	m.state = managerStateStopped
	return err
}

func (m *Manager) loadProbes(target *process.TargetDetails) error {
	// Remove resource limits for kernels <5.11.
	if err := rlimitRemoveMemlock(); err != nil {
		return err
	}

	exe, err := openExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}
	m.exe = exe

	if err := m.mount(target); err != nil {
		return err
	}

	// Load probes
	for name, i := range m.probes {
		if isProbeEnabled(name, m.currentConfig) {
			m.logger.Info("loading probe", "name", name)
			err := i.Load(exe, target, m.currentConfig.SamplingConfig)
			if err != nil {
				m.logger.Error("error while loading probes, cleaning up", "error", err, "name", name)
				return errors.Join(err, m.cleanup(target))
			}
		}
	}

	m.logger.Debug("loaded probes to memory", "total_probes", len(m.probes))
	return nil
}

func (m *Manager) mount(target *process.TargetDetails) error {
	if target.AllocationDetails != nil {
		m.logger.Debug("Mounting bpffs", "allocations_details", target.AllocationDetails)
	} else {
		m.logger.Debug("Mounting bpffs")
	}
	return bpffsMount(target)
}

func (m *Manager) cleanup(target *process.TargetDetails) error {
	ctx := context.Background()
	err := m.cp.Shutdown(context.Background())
	for _, i := range m.probes {
		err = errors.Join(err, i.Close())
	}

	// Wait for all probes to close so we know there is no more telemetry being
	// generated before stopping (and flushing) the Controller.
	if m.otelController != nil {
		err = errors.Join(err, m.otelController.Shutdown(ctx))
	}

	m.logger.Debug("Cleaning bpffs")
	return errors.Join(err, bpffsCleanup(target))
}

func (m *Manager) availableProbes() []probe.Probe {
	p := []probe.Probe{
		grpcClient.New(m.logger, m.version),
		grpcServer.New(m.logger, m.version),
		httpServer.New(m.logger, m.version),
		httpClient.New(m.logger, m.version),
		dbSql.New(m.logger, m.version),
		kafkaProducer.New(m.logger, m.version),
		kafkaConsumer.New(m.logger, m.version),
		autosdk.New(m.logger),
	}

	if m.globalImpl {
		p = append(p, otelTraceGlobal.New(m.logger))
	}

	return p
}

func (m *Manager) registerProbes() error {
	for _, p := range m.availableProbes() {
		if err := m.registerProbe(p); err != nil {
			return err
		}
	}
	return nil
}
