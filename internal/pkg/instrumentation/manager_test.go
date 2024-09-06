//go:build !multi_kernel_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/go-logr/stdr"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/config"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
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
			Libraries:         map[string]*version.Version{},
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
			Libraries:         map[string]*version.Version{},
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
			Libraries:         map[string]*version.Version{},
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
			Libraries:         map[string]*version.Version{},
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
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")

	m, err := NewManager(logger, nil, true, nil, config.NewNoopProvider(nil))
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
	bpffsMount = func(*process.TargetDetails) error { return nil }
	t.Cleanup(func() { bpffsMount = origBpffsMount })

	origBpffsCleanup := bpffsCleanup
	bpffsCleanup = func(*process.TargetDetails) error { return nil }
	t.Cleanup(func() { bpffsCleanup = origBpffsCleanup })
}

type shutdownTracerProvider struct {
	trace.TracerProvider

	called bool
}

func (tp *shutdownTracerProvider) Shutdown(context.Context) error {
	tp.called = true
	return nil
}

func TestRunStopping(t *testing.T) {
	probeStop := make(chan struct{})
	p := newSlowProbe(probeStop)

	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")

	tp := new(shutdownTracerProvider)
	ctrl, err := opentelemetry.NewController(logger, tp, "")
	require.NoError(t, err)

	m := &Manager{
		otelController: ctrl,
		logger:         logger.WithName("Manager"),
		probes:         map[probe.ID]probe.Probe{{}: p},
		eventCh:        make(chan *probe.Event),
		cp:             config.NewNoopProvider(nil),
	}

	mockExeAndBpffs(t)

	ctx, stopCtx := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- m.Run(ctx, &process.TargetDetails{PID: 1000}) }()

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

func (p slowProbe) Load(*link.Executable, *process.TargetDetails, config.Sampler) error {
	return nil
}

func (p slowProbe) Run(c chan<- *probe.Event) {
}

func (p slowProbe) Close() error {
	p.closeSignal <- struct{}{}
	<-p.stop
	return nil
}

type noopProbe struct {
	loaded, running, closed bool
}

var _ probe.Probe = (*noopProbe)(nil)

func (p *noopProbe) Load(*link.Executable, *process.TargetDetails, config.Sampler) error {
	p.loaded = true
	return nil
}

func (p *noopProbe) Run(c chan<- *probe.Event) {
	p.running = true
}

func (p *noopProbe) Close() error {
	p.closed = true
	p.loaded = false
	p.running = false
	return nil
}

func (p *noopProbe) Manifest() probe.Manifest {
	return probe.Manifest{}
}

type dummyProvider struct {
	initial config.InstrumentationConfig
	ch      chan config.InstrumentationConfig
}

func newDummyProvider(initialConfig config.InstrumentationConfig) config.Provider {
	return &dummyProvider{
		ch:      make(chan config.InstrumentationConfig),
		initial: initialConfig,
	}
}

func (p *dummyProvider) InitialConfig(_ context.Context) config.InstrumentationConfig {
	return p.initial
}

func (p *dummyProvider) Watch() <-chan config.InstrumentationConfig {
	return p.ch
}

func (p *dummyProvider) Shutdown(_ context.Context) error {
	close(p.ch)
	return nil
}

func (p *dummyProvider) sendConfig(c config.InstrumentationConfig) {
	p.ch <- c
}

func TestConfigProvider(t *testing.T) {
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")
	loadedIndicator := make(chan struct{})

	netHTTPClientProbeID := probe.ID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindClient}
	netHTTPServerProbeID := probe.ID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindServer}
	somePackageProducerProbeID := probe.ID{InstrumentedPkg: "some/package", SpanKind: trace.SpanKindProducer}

	netHTTPClientLibID := config.InstrumentationLibraryID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindClient}
	netHTTPLibID := config.InstrumentationLibraryID{InstrumentedPkg: "net/http"}
	falseVal := false

	m := &Manager{
		logger: logger.WithName("Manager"),
		probes: map[probe.ID]probe.Probe{
			netHTTPClientProbeID:       &noopProbe{},
			netHTTPServerProbeID:       &noopProbe{},
			somePackageProducerProbeID: &noopProbe{},
		},
		eventCh: make(chan *probe.Event),
		cp: newDummyProvider(config.InstrumentationConfig{
			InstrumentationLibraryConfigs: map[config.InstrumentationLibraryID]config.InstrumentationLibrary{
				netHTTPClientLibID: {TracesEnabled: &falseVal},
			},
		}),
		loadedIndicator: loadedIndicator,
	}

	mockExeAndBpffs(t)
	runCtx, cancel := context.WithCancel(context.Background())
	go func() { _ = m.Run(runCtx, &process.TargetDetails{PID: 1000}) }()
	assert.Eventually(t, func() bool {
		select {
		case <-loadedIndicator:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	probeRunning := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return p.loaded && p.running
	}

	probePending := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return !p.loaded && !p.running
	}

	probeClosed := func(id probe.ID) bool {
		p := m.probes[id].(*noopProbe)
		return p.closed
	}

	assert.True(t, probePending(netHTTPClientProbeID))
	assert.True(t, probeRunning(netHTTPServerProbeID))
	assert.True(t, probeRunning(somePackageProducerProbeID))

	// Send a new config that enables the net/http client probe by removing the explicit disable
	m.cp.(*dummyProvider).sendConfig(config.InstrumentationConfig{})
	assert.Eventually(t, func() bool {
		return probeRunning(netHTTPClientProbeID)
	}, time.Second, 10*time.Millisecond)
	assert.True(t, probeRunning(netHTTPServerProbeID))
	assert.True(t, probeRunning(somePackageProducerProbeID))

	// Send a new config that disables the net/http client and server probes
	m.cp.(*dummyProvider).sendConfig(config.InstrumentationConfig{
		InstrumentationLibraryConfigs: map[config.InstrumentationLibraryID]config.InstrumentationLibrary{
			netHTTPLibID: {TracesEnabled: &falseVal},
		},
	})
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID)
	}, time.Second, 10*time.Millisecond)
	assert.True(t, probeRunning(somePackageProducerProbeID))

	// Send a new config the disables all probes by default
	m.cp.(*dummyProvider).sendConfig(config.InstrumentationConfig{
		DefaultTracesDisabled: true,
	})
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID) &&
			probeClosed(somePackageProducerProbeID) && !probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	// Send a new config that enables all probes by default
	m.cp.(*dummyProvider).sendConfig(config.InstrumentationConfig{})
	assert.Eventually(t, func() bool {
		return probeRunning(netHTTPClientProbeID) &&
			probeRunning(netHTTPServerProbeID) &&
			probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	cancel()
	assert.Eventually(t, func() bool {
		return probeClosed(netHTTPClientProbeID) && !probeRunning(netHTTPClientProbeID) &&
			probeClosed(netHTTPServerProbeID) && !probeRunning(netHTTPServerProbeID) &&
			probeClosed(somePackageProducerProbeID) && !probeRunning(somePackageProducerProbeID)
	}, time.Second, 10*time.Millisecond)

	// Send a config to enable all probes, but the manager is stopped - this should panic
	assert.Panics(t, func() {
		m.cp.(*dummyProvider).sendConfig(config.InstrumentationConfig{})
	})
}
