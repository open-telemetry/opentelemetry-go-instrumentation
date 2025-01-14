//go:build !ebpf_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

func TestProbeFiltering(t *testing.T) {
	ver, err := version.NewVersion("1.20.0")
	assert.NoError(t, err)

	t.Run("empty target details", func(t *testing.T) {
		m := fakeManager(t)

		td := process.TargetDetails{
			PID:               1,
			Functions:         []*binary.Func{},
			GoVersion:         ver,
			Modules:           map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 0, len(m.probes))
	})

	t.Run("only HTTP client target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			{Name: "net/http.(*Transport).roundTrip"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Modules:           map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 1, len(m.probes)) // one function, single probe
	})

	t.Run("HTTP server and client target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			{Name: "net/http.(*Transport).roundTrip"},
			{Name: "net/http.serverHandler.ServeHTTP"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Modules:           map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 2, len(m.probes))
	})

	t.Run("HTTP server and client dependent function only target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			// writeSubset depends on "net/http.(*Transport).roundTrip", it should be ignored without roundTrip
			{Name: "net/http.Header.writeSubset"},
			{Name: "net/http.serverHandler.ServeHTTP"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Modules:           map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 1, len(m.probes))
	})
}

func TestDependencyChecks(t *testing.T) {
	m := fakeManager(t)

	t.Run("Dependent probes match", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:    "A",
				DependsOn: nil,
			},
			{
				Symbol:    "B",
				DependsOn: []string{"A"},
			},
		}

		assert.Nil(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Second dependent missing", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:    "A",
				DependsOn: nil,
			},
			{
				Symbol:    "B",
				DependsOn: []string{"A", "C"},
			},
		}

		assert.NotNil(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Second dependent present", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:    "A",
				DependsOn: nil,
			},
			{
				Symbol:    "B",
				DependsOn: []string{"A", "C"},
			},
			{
				Symbol:    "C",
				DependsOn: []string{"A"},
			},
		}

		assert.Nil(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Dependent wrong", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:    "A",
				DependsOn: nil,
			},
			{
				Symbol:    "B",
				DependsOn: []string{"A1"},
			},
		}

		assert.NotNil(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Two probes without dependents", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:    "A",
				DependsOn: nil,
			},
			{
				Symbol:    "B",
				DependsOn: []string{},
			},
		}

		assert.Nil(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})
}

func fakeManager(t *testing.T) *Manager {
	m, err := NewManager(slog.Default(), nil, true, NewNoopConfigProvider(nil), "")
	assert.NoError(t, err)
	assert.NotNil(t, m)

	return m
}

func mockExeAndBpffs(t *testing.T) {
	origOpenExecutable := openExecutable
	openExecutable = func(string) (*link.Executable, error) { return &link.Executable{}, nil }
	t.Cleanup(func() { openExecutable = origOpenExecutable })

	origRlimitRemoveMemlock := rlimitRemoveMemlock
	rlimitRemoveMemlock = func() error { return nil }
	t.Cleanup(func() { rlimitRemoveMemlock = origRlimitRemoveMemlock })

	origBpffsMount := bpffsMount
	bpffsMount = func(td *process.TargetDetails) error {
		if td == nil {
			return errors.New("target is nil in Mount")
		}
		return nil
	}
	t.Cleanup(func() { bpffsMount = origBpffsMount })

	origBpffsCleanup := bpffsCleanup
	bpffsCleanup = func(td *process.TargetDetails) error {
		if td == nil {
			return errors.New("target is nil in Cleanup")
		}
		return nil
	}
	t.Cleanup(func() { bpffsCleanup = origBpffsCleanup })
}

type shutdownTracerProvider struct {
	noop.TracerProvider

	called bool
}

func (tp *shutdownTracerProvider) Shutdown(context.Context) error {
	tp.called = true
	return nil
}

func TestRunStoppingByContext(t *testing.T) {
	probeStop := make(chan struct{})
	p := newSlowProbe(probeStop)

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(slog.Default(), tp)
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         slog.Default(),
		probes:         map[probe.ID]probe.Probe{{}: p},
		cp:             NewNoopConfigProvider(nil),
	}

	mockExeAndBpffs(t)

	ctx, stopCtx := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	err = m.Load(ctx, &process.TargetDetails{PID: 1000})
	require.NoError(t, err)

	go func() { errCh <- m.Run(ctx) }()

	assert.NotPanics(t, func() {
		stopCtx()
		assert.Eventually(t, func() bool {
			select {
			case <-p.closeSignal:
				return true
			default:
				return false
			}
		}, time.Second, 10*time.Millisecond)
		close(probeStop)
	})

	assert.Eventually(t, func() bool {
		select {
		case err = <-errCh:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, err, context.Canceled, "Stopping Run error")
	assert.True(t, tp.called, "Controller not stopped")
}

func TestRunStoppingByStop(t *testing.T) {
	p := noopProbe{}

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(slog.Default(), tp)
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         slog.Default(),
		probes:         map[probe.ID]probe.Probe{{}: &p},
		cp:             NewNoopConfigProvider(nil),
	}

	mockExeAndBpffs(t)

	ctx := context.Background()
	errCh := make(chan error, 1)

	err = m.Load(ctx, &process.TargetDetails{PID: 1000})
	require.NoError(t, err)

	time.AfterFunc(100*time.Millisecond, func() {
		err := m.Stop()
		require.NoError(t, err)
	})
	go func() { errCh <- m.Run(ctx) }()

	assert.Eventually(t, func() bool {
		select {
		case <-errCh:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, err, nil)
	assert.True(t, tp.called, "Controller not stopped")
	assert.True(t, p.closed.Load(), "Probe not closed")
}

type slowProbe struct {
	probe.Probe

	closeSignal chan struct{}
	stop        chan struct{}
}

func newSlowProbe(stop chan struct{}) slowProbe {
	return slowProbe{
		closeSignal: make(chan struct{}),
		stop:        stop,
	}
}

func (p slowProbe) Load(*link.Executable, *process.TargetDetails, *sampling.Config) error {
	return nil
}

func (p slowProbe) Run(func(ptrace.ScopeSpans)) {
}

func (p slowProbe) Close() error {
	p.closeSignal <- struct{}{}
	<-p.stop
	return nil
}

type noopProbe struct {
	loaded, running, closed atomic.Bool
}

var _ probe.Probe = (*noopProbe)(nil)

func (p *noopProbe) Load(*link.Executable, *process.TargetDetails, *sampling.Config) error {
	p.loaded.Store(true)
	return nil
}

func (p *noopProbe) Run(func(ptrace.ScopeSpans)) {
	p.running.Store(true)
}

func (p *noopProbe) Close() error {
	p.closed.Store(true)
	p.loaded.Store(false)
	p.running.Store(false)
	return nil
}

func (p *noopProbe) Manifest() probe.Manifest {
	return probe.Manifest{}
}

type dummyProvider struct {
	initial Config
	ch      chan Config
}

func newDummyProvider(initialConfig Config) ConfigProvider {
	return &dummyProvider{
		ch:      make(chan Config),
		initial: initialConfig,
	}
}

func (p *dummyProvider) InitialConfig(_ context.Context) Config {
	return p.initial
}

func (p *dummyProvider) Watch() <-chan Config {
	return p.ch
}

func (p *dummyProvider) Shutdown(_ context.Context) error {
	close(p.ch)
	return nil
}

func (p *dummyProvider) sendConfig(c Config) {
	p.ch <- c
}

func TestConfigProvider(t *testing.T) {
	netHTTPClientProbeID := probe.ID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindClient}
	netHTTPServerProbeID := probe.ID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindServer}
	somePackageProducerProbeID := probe.ID{InstrumentedPkg: "some/package", SpanKind: trace.SpanKindProducer}

	netHTTPClientLibID := LibraryID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindClient}
	netHTTPLibID := LibraryID{InstrumentedPkg: "net/http"}
	falseVal := false

	m := &Manager{
		logger: slog.Default(),
		probes: map[probe.ID]probe.Probe{
			netHTTPClientProbeID:       &noopProbe{},
			netHTTPServerProbeID:       &noopProbe{},
			somePackageProducerProbeID: &noopProbe{},
		},
		cp: newDummyProvider(Config{
			InstrumentationLibraryConfigs: map[LibraryID]Library{
				netHTTPClientLibID: {TracesEnabled: &falseVal},
			},
		}),
	}

	mockExeAndBpffs(t)
	runCtx, cancel := context.WithCancel(context.Background())

	err := m.Load(runCtx, &process.TargetDetails{PID: 1000})
	require.NoError(t, err)

	runErr := make(chan error, 1)

	go func() { runErr <- m.Run(runCtx) }()

	probeRunning := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return p.loaded.Load() && p.running.Load()
	}

	probePending := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return !p.loaded.Load() && !p.running.Load()
	}

	probeClosed := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return p.closed.Load()
	}

	assert.True(t, probePending(netHTTPClientProbeID))
	assert.Eventually(t, func() bool {
		return probeRunning(netHTTPServerProbeID)
	}, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool {
		return probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	// Send a new config that enables the net/http client probe by removing the explicit disable
	m.cp.(*dummyProvider).sendConfig(Config{})
	assert.Eventually(t, func() bool {
		return probeRunning(netHTTPClientProbeID)
	}, time.Second, 10*time.Millisecond)
	assert.True(t, probeRunning(netHTTPServerProbeID))
	assert.True(t, probeRunning(somePackageProducerProbeID))

	// Send a new config that disables the net/http client and server probes
	m.cp.(*dummyProvider).sendConfig(Config{
		InstrumentationLibraryConfigs: map[LibraryID]Library{
			netHTTPLibID: {TracesEnabled: &falseVal},
		},
	})
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID)
	}, time.Second, 10*time.Millisecond)
	assert.True(t, probeRunning(somePackageProducerProbeID))

	// Send a new config the disables all probes by default
	m.cp.(*dummyProvider).sendConfig(Config{
		DefaultTracesDisabled: true,
	})
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID) &&
			probeClosed(somePackageProducerProbeID) && !probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	// Send a new config that enables all probes by default
	m.cp.(*dummyProvider).sendConfig(Config{})
	assert.Eventually(t, func() bool {
		return probeRunning(netHTTPClientProbeID) &&
			probeRunning(netHTTPServerProbeID) &&
			probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	cancel()
	assert.Eventually(t, func() bool {
		select {
		case <-runErr:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID) &&
			probeClosed(somePackageProducerProbeID) && !probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	// Send a config to enable all probes, but the manager is stopped - this should panic
	assert.Panics(t, func() {
		m.cp.(*dummyProvider).sendConfig(Config{})
	})
}

type hangingProbe struct {
	probe.Probe

	closeReturned chan struct{}
}

func newHangingProbe() *hangingProbe {
	return &hangingProbe{closeReturned: make(chan struct{})}
}

func (p *hangingProbe) Load(*link.Executable, *process.TargetDetails, *sampling.Config) error {
	return nil
}

func (p *hangingProbe) Run(handle func(ptrace.ScopeSpans)) {
	<-p.closeReturned
	// Write after Close has returned.
	handle(ptrace.NewScopeSpans())
}

func (p *hangingProbe) Close() error {
	defer close(p.closeReturned)
	return nil
}

func TestRunStopDeadlock(t *testing.T) {
	// Regression test for #1228.
	p := newHangingProbe()

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(slog.Default(), tp)
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         slog.Default(),
		probes:         map[probe.ID]probe.Probe{{}: p},
		cp:             NewNoopConfigProvider(nil),
	}

	mockExeAndBpffs(t)

	ctx, stopCtx := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	err = m.Load(ctx, &process.TargetDetails{PID: 1000})
	require.NoError(t, err)

	go func() { errCh <- m.Run(ctx) }()

	assert.NotPanics(t, func() {
		stopCtx()
		assert.Eventually(t, func() bool {
			select {
			case <-p.closeReturned:
				return true
			default:
				return false
			}
		}, time.Second, 10*time.Millisecond)
	})

	assert.Eventually(t, func() bool {
		select {
		case err = <-errCh:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, err, context.Canceled, "Stopping Run error")
	assert.True(t, tp.called, "Controller not stopped")
}

func TestStopBeforeLoad(t *testing.T) {
	p := noopProbe{}

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(slog.Default(), tp)
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         slog.Default(),
		probes:         map[probe.ID]probe.Probe{{}: &p},
		cp:             NewNoopConfigProvider(nil),
	}

	mockExeAndBpffs(t)

	err = m.Stop()
	require.NoError(t, err)
}

func TestStopBeforeRun(t *testing.T) {
	p := noopProbe{}

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(slog.Default(), tp)
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         slog.Default(),
		probes:         map[probe.ID]probe.Probe{{}: &p},
		cp:             NewNoopConfigProvider(nil),
	}

	mockExeAndBpffs(t)

	err = m.Load(context.Background(), &process.TargetDetails{PID: 1000})
	require.NoError(t, err)
	require.True(t, p.loaded.Load())

	err = m.Stop()
	require.NoError(t, err)
	require.True(t, p.closed.Load())
	require.False(t, p.running.Load())
}
