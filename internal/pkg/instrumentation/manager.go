// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package instrumentation provides functionality to manage instrumentation
// using eBPF for Go programs.
package instrumentation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/pipeline"
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
	probes          map[probe.ID]*probeReference
	handler         *pipeline.Handler
	cp              ConfigProvider
	exe             *link.Executable
	proc            *process.Info
	stop            context.CancelCauseFunc
	runningProbesWG sync.WaitGroup
	currentConfig   Config
	probeMu         sync.Mutex
	state           managerState
	stateMu         sync.RWMutex
}

// probeReference is used by the Manager to track an initialized reference
// to a Probe and its related resources such as its ebpf.Collection and io.Closers.
type probeReference struct {
	probe      probe.Probe
	collection *ebpf.Collection
	closers    []io.Closer
}

// NewManager returns a new [Manager].
func NewManager(
	logger *slog.Logger,
	h *pipeline.Handler,
	pid process.ID,
	cp ConfigProvider,
	probes ...probe.Probe,
) (*Manager, error) {
	m := &Manager{
		logger:  logger,
		probes:  make(map[probe.ID]*probeReference),
		handler: h,
		cp:      cp,
	}

	funcs := make(map[string]any)
	for _, p := range probes {
		if err := m.registerProbe(p); err != nil {
			return nil, err
		}

		for _, u := range p.GetUprobes() {
			funcs[u.Sym] = nil
		}
	}

	var err error
	m.proc, err = process.NewInfo(pid, funcs)
	if err != nil {
		return nil, err
	}

	m.logger.Info("loaded process info", "process", m.proc)

	m.filterUnusedProbes()

	return m, nil
}

func (m *Manager) validateProbeDependents(id probe.ID, uprobes []*probe.Uprobe) error {
	// Validate that dependent probes point to real standalone probes.
	funcsMap := make(map[string]struct{}, len(uprobes))
	for _, u := range uprobes {
		funcsMap[u.Sym] = struct{}{}
	}

	for _, u := range uprobes {
		for _, d := range u.DependsOn {
			if _, exists := funcsMap[d]; !exists {
				return fmt.Errorf(
					"library %s has declared a dependent function %s for probe %s which does not exist, aborting",
					id,
					d,
					u.Sym,
				)
			}
		}
	}

	return nil
}

func (m *Manager) registerProbe(p probe.Probe) error {
	id := p.GetID()
	if _, exists := m.probes[id]; exists {
		return fmt.Errorf("library %s registered twice, aborting", id)
	}

	if err := m.validateProbeDependents(id, p.GetUprobes()); err != nil {
		return err
	}

	m.probes[id] = &probeReference{
		probe: p,
	}
	return nil
}

// filterUnusedProbes filterers probes whose functions are already instrumented
// out of the Manager.
func (m *Manager) filterUnusedProbes() {
	existingFuncMap := make(map[string]struct{}, len(m.proc.Functions))
	for _, f := range m.proc.Functions {
		existingFuncMap[f.Name] = struct{}{}
	}

	for name, inst := range m.probes {
		funcsFound := false
		for _, u := range inst.probe.GetUprobes() {
			if len(u.DependsOn) == 0 {
				if _, exists := existingFuncMap[u.Sym]; exists {
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
	if m.proc == nil {
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
			err = errors.Join(err, m.closeProbe(p))
			continue
		}

		if !currentlyEnabled && newEnabled {
			m.logger.Info("Enabling probe", "id", id)
			collection, loadErr := m.loadProbeCollection(p.probe, c)
			if loadErr != nil {
				err = errors.Join(err, loadErr)
				continue
			}
			p.collection = collection

			closers, upErr := m.loadAndConfigureUprobesFromProbe(p, c.SamplingConfig)
			if upErr != nil {
				err = errors.Join(err, upErr)
				continue
			}
			p.closers = append(p.closers, closers...)

			m.runProbe(p.probe)
		}
	}

	return nil
}

func (m *Manager) runProbe(p probe.Probe) {
	m.runningProbesWG.Add(1)
	go func(ap probe.Probe) {
		defer m.runningProbesWG.Done()
		ap.Run(m.handler)
	}(p)
}

func (m *Manager) ConfigLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c, ok := <-m.cp.Watch():
			if !ok {
				m.logger.Info(
					"Configuration provider closed, configuration updates will no longer be received",
				)
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

func (m *Manager) Load(ctx context.Context) error {
	if len(m.probes) == 0 {
		return errors.New("no instrumentation for target process")
	}
	if m.cp == nil {
		return errors.New("no config provider set")
	}
	if m.proc == nil {
		return errors.New(
			"target details not set - load is called on non-initialized instrumentation",
		)
	}
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if m.state == managerStateRunning {
		return errors.New("manager is already running, load is not allowed")
	}

	m.currentConfig = m.cp.InitialConfig(ctx)
	err := m.loadProbes()
	if err != nil {
		return err
	}

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
			m.runProbe(p.probe)
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
	err := m.cleanup()

	// Wait for all probes to stop.
	m.runningProbesWG.Wait()

	m.state = managerStateStopped
	return err
}

func (m *Manager) loadProbes() error {
	// Remove resource limits for kernels <5.11.
	if err := rlimitRemoveMemlock(); err != nil {
		return err
	}

	exe, err := openExecutable(m.proc.ID.ExePath())
	if err != nil {
		return err
	}
	m.exe = exe

	m.logger.Debug("Mounting bpffs")
	if err := bpffsMount(m.proc); err != nil {
		return err
	}

	// Load probes
	for name, i := range m.probes {
		if isProbeEnabled(name, m.currentConfig) {
			collection, err := m.loadProbeCollection(i.probe, m.currentConfig)
			if err != nil {
				m.logger.Error(
					"error while loading probe collection, cleaning up",
					"error",
					err,
					"probe",
					name,
				)
				return errors.Join(err, m.cleanup())
			}
			i.collection = collection

			closers, err := m.loadAndConfigureUprobesFromProbe(i, m.currentConfig.SamplingConfig)
			if err != nil {
				m.logger.Error(
					"error while loading uprobes from probe, cleaning up",
					"error",
					err,
					"probe",
					name,
				)
				return errors.Join(err, m.cleanup())
			}
			i.closers = append(i.closers, closers...)
		}
	}

	m.logger.Debug("loaded probes to memory", "total_probes", len(m.probes))
	return nil
}

func (m *Manager) loadProbeCollection(p probe.Probe, cfg Config) (*ebpf.Collection, error) {
	m.logger.Info("loading probe", "name", p.GetID())

	spec, err := p.Spec()
	if err != nil {
		return nil, err
	}

	err = m.injectProbeConsts(p, spec)
	if err != nil {
		return nil, err
	}

	collection, err := initializeEBPFCollection(spec, m.proc)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func (m *Manager) injectProbeConsts(i probe.Probe, spec *ebpf.CollectionSpec) error {
	var err error
	var opts []inject.Option
	for _, cnst := range i.GetConsts() {
		if l, ok := cnst.(probe.SetLogger); ok {
			cnst = l.SetLogger(m.logger)
		}

		o, e := cnst.InjectOption(m.proc)
		err = errors.Join(err, e)
		if e == nil && o != nil {
			opts = append(opts, o)
		}
	}
	if err != nil {
		return err
	}

	return inject.Constants(spec, opts...)
}

func (m *Manager) loadAndConfigureUprobesFromProbe(i *probeReference, sampler *sampling.Config) ([]io.Closer, error) {
	var closers []io.Closer
	for _, up := range i.probe.GetUprobes() {
		var skip bool
		for _, pc := range up.PackageConstraints {
			if pc.Constraints.Check(m.proc.Modules[pc.Package]) {
				continue
			}

			var logFn func(string, ...any)
			switch pc.FailureMode {
			case probe.FailureModeIgnore:
				logFn = m.logger.Debug
			case probe.FailureModeWarn:
				logFn = m.logger.Warn
			default:
				// Unknown and FailureModeError.
				return nil, fmt.Errorf(
					"uprobe %s package constraint (%s) not met, version %v",
					up.Sym,
					pc.Constraints.String(),
					m.proc.Modules[pc.Package])
			}

			logFn(
				"package constraint not meet, skipping uprobe",
				"probe", i.probe.GetID(),
				"symbol", up.Sym,
				"package", pc.Package,
				"constraint", pc.Constraints.String(),
				"version", m.proc.Modules[pc.Package],
			)

			skip = true
			break
		}
		if skip {
			continue
		}

		err := m.loadUprobe(up, i.collection)
		if err != nil {
			var logFn func(string, ...any)
			switch up.FailureMode {
			case probe.FailureModeIgnore:
				logFn = m.logger.Debug
			case probe.FailureModeWarn:
				logFn = m.logger.Warn
			default:
				// Unknown and FailureModeError.
				return nil, err
			}
			logFn("failed to load uprobe", "probe", i.probe.GetID(), "symbol", up.Sym, "error", err)
			continue
		}

		closers = append(closers, up)
	}

	reader, err := i.probe.InitStartupConfig(i.collection, m.currentConfig.SamplingConfig)
	if err != nil {
		return nil, err
	}
	closers = append(closers, reader)

	return closers, nil
}

func (m *Manager) loadUprobe(u *probe.Uprobe, c *ebpf.Collection) error {
	offset, err := m.proc.GetFunctionOffset(u.Sym)
	if err != nil {
		return err
	}

	var closers []io.Closer

	if u.EntryProbe != "" {
		entryProg, ok := c.Programs[u.EntryProbe]
		if !ok {
			return fmt.Errorf("entry probe %s not found", u.EntryProbe)
		}
		opts := &link.UprobeOptions{Address: offset, PID: int(m.proc.ID)}
		l, err := m.exe.Uprobe("", entryProg, opts)
		if err != nil {
			return err
		}
		closers = append(closers, l)
	}

	if u.ReturnProbe != "" {
		retProg, ok := c.Programs[u.ReturnProbe]
		if !ok {
			return fmt.Errorf("return probe %s not found", u.ReturnProbe)
		}
		retOffsets, err := m.proc.GetFunctionReturns(u.Sym)
		if err != nil {
			return err
		}

		for _, ret := range retOffsets {
			opts := &link.UprobeOptions{Address: ret, PID: int(m.proc.ID)}
			l, err := m.exe.Uprobe("", retProg, opts)
			if err != nil {
				return err
			}
			closers = append(closers, l)
		}
	}

	old := u.Closers.Swap(&closers)
	if old != nil {
		// load called twice without calling Close. Try and handle gracefully.
		var err error
		for _, closer := range *old {
			err = errors.Join(err, closer.Close())
		}
		return err
	}

	return nil
}

func (m *Manager) closeProbe(p *probeReference) error {
	if p.collection != nil {
		p.collection.Close()
	}

	var err error
	for _, c := range p.closers {
		err = errors.Join(err, c.Close())
	}
	if err == nil {
		m.logger.Debug("Closed", "Probe", p.probe.GetID())
	}
	return err
}

func (m *Manager) cleanup() error {
	err := m.cp.Shutdown(context.Background())
	for _, i := range m.probes {
		err = errors.Join(err, m.closeProbe(i))
	}

	m.logger.Debug("Cleaning bpffs")
	return errors.Join(err, bpffsCleanup(m.proc))
}

// initializeEBPFCollection loads eBPF objects from the given spec and returns a collection corresponding to the spec.
// If the environment variable OTEL_GO_AUTO_SHOW_VERIFIER_LOG is set to true, the verifier log will be printed.
func initializeEBPFCollection(
	spec *ebpf.CollectionSpec,
	proc *process.Info,
) (*ebpf.Collection, error) {
	collectionOpts := &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(proc),
		},
	}

	// Getting full verifier log is expensive, so we only do it if the user explicitly asks for it.
	showVerifierLogs := utils.ShouldShowVerifierLogs()
	if showVerifierLogs {
		collectionOpts.Programs.LogLevel = ebpf.LogLevelInstruction | ebpf.LogLevelBranch | ebpf.LogLevelStats
	}

	c, err := ebpf.NewCollectionWithOptions(spec, *collectionOpts)
	if err != nil && showVerifierLogs {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			fmt.Printf("Verifier log: %-100v\n", ve)
		}
	}

	return c, err
}
