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
	probes          map[probe.ID]probe.BaseProbe
	otelController  *opentelemetry.Controller
	cp              ConfigProvider
	exe             *link.Executable
	td              *process.TargetDetails
	stop            context.CancelCauseFunc
	runningProbesWG sync.WaitGroup
	currentConfig   Config
	probeMu         sync.Mutex
	state           managerState
	stateMu         sync.RWMutex
	relevantFuncs   map[string]interface{}
}

// NewManager returns a new [Manager].
func NewManager(logger *slog.Logger, otelController *opentelemetry.Controller, probes []probe.BaseProbe, cp ConfigProvider, version string) (*Manager, error) {
	m := &Manager{
		logger:         logger,
		version:        version,
		probes:         make(map[probe.ID]probe.BaseProbe),
		otelController: otelController,
		cp:             cp,
		relevantFuncs:  make(map[string]interface{}),
	}

	for _, p := range probes {
		switch probeObj := p.(type) {
		case probe.GoLibraryTelemetryProbe:
			id := probeObj.Manifest().Id
			if _, exists := m.probes[id]; exists {
				return nil, fmt.Errorf("library %s registered twice, aborting", id)
			}

			if err := m.validateProbeDependents(id, probeObj.Manifest().Symbols); err != nil {
				return nil, err
			}

			m.probes[id] = p

		default:
			return nil, fmt.Errorf("unknown probe type")
		}
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

		// if no dependency issues, add the symbol to the manager's relevant funcs
		m.relevantFuncs[s.Symbol] = nil
	}

	return nil
}

// GetRelevantFuncs returns the instrumented functions for all managed probes.
func (m *Manager) GetRelevantFuncs() map[string]interface{} {
	return m.relevantFuncs
}

// FilterUnusedProbesForTarget filters probes whose functions are not instrumented
// out of the Manager, and updates instrumented probes with Target Details.
func (m *Manager) FilterUnusedProbesForTarget(target *process.TargetDetails) {
	existingFuncMap := make(map[string]interface{})
	for _, f := range target.Functions {
		existingFuncMap[f.Name] = nil
	}

	for name, inst := range m.probes {
		switch p := inst.(type) {
		case probe.GoLibraryTelemetryProbe:
			// Filter Probe if unused in target process
			funcsFound := false
			for _, s := range p.Manifest().Symbols {
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
				continue
			}

			// If Probe is used, pass target details to Probe
			p.TargetConfig().TargetDetails = target

		default:
			continue
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
		if runnableProbe, ok := p.(probe.RunnableProbe); ok {
			currentlyEnabled := isProbeEnabled(id, m.currentConfig)
			newEnabled := isProbeEnabled(id, c)

			if currentlyEnabled && !newEnabled {
				m.logger.Info("Disabling probe", "id", id)
				err = errors.Join(err, runnableProbe.Close())
				continue
			}

			if !currentlyEnabled && newEnabled {
				m.logger.Info("Enabling probe", "id", id)

				if tracingProbe, ok := p.(probe.TracingProbe); ok {
					tracingProbe.TraceConfig().SamplingConfig = c.SamplingConfig
				}

				err = errors.Join(err, runnableProbe.Load())
				if err != nil {
					continue
				}
				err = errors.Join(err, runnableProbe.Attach())
				if err == nil {
					m.runningProbesWG.Add(1)
					go func(ap probe.RunnableProbe) {
						defer m.runningProbesWG.Done()
						ap.Run()
					}(runnableProbe)
				}
				continue
			}
		}
	}

	return err
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
			if tp, ok := p.(probe.RunnableProbe); ok {
				m.runningProbesWG.Add(1)
				go func(ap probe.RunnableProbe) {
					defer m.runningProbesWG.Done()
					ap.Run()
				}(tp)
			}
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
			if p, ok := i.(probe.TracingProbe); ok {
				p.TraceConfig().SamplingConfig = m.currentConfig.SamplingConfig
			}
			if p, ok := i.(probe.GoLibraryTelemetryProbe); ok {
				p.TargetConfig().Executable = exe
			}

			err := i.Load()
			if err != nil {
				m.logger.Error("error while loading probes, cleaning up", "error", err, "name", name)
				return errors.Join(err, m.cleanup(target))
			}

			err = i.Attach()
			if err != nil {
				m.logger.Error("error while attaching probes, cleaning up", "error", err, "name", name)
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

	// Shut down TelemetryProbes
	for _, i := range m.probes {
		if p, ok := i.(probe.RunnableProbe); ok {
			err = errors.Join(err, p.Close())
		}
	}

	// Wait for all probes to close so we know there is no more telemetry being
	// generated before stopping (and flushing) the Controller.
	if m.otelController != nil {
		err = errors.Join(err, m.otelController.Shutdown(ctx))
	}

	m.logger.Debug("Cleaning bpffs")
	return errors.Join(err, bpffsCleanup(target))
}
