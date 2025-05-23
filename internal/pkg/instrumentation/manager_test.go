//go:build !ebpf_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/process/binary"
	"go.opentelemetry.io/auto/pipeline"
)

func TestProbeFiltering(t *testing.T) {
	t.Run("empty target details", func(t *testing.T) {
		m := fakeManager()
		assert.Empty(t, m.probes)
	})

	t.Run("only HTTP client target details", func(t *testing.T) {
		m := fakeManager("net/http.(*Transport).roundTrip")
		assert.Len(t, m.probes, 1) // one function, single probe
	})

	t.Run("HTTP server and client target details", func(t *testing.T) {
		m := fakeManager(
			"net/http.(*Transport).roundTrip",
			"net/http.serverHandler.ServeHTTP",
		)
		assert.Len(t, m.probes, 2)
	})

	t.Run("HTTP server and client dependent function only target details", func(t *testing.T) {
		m := fakeManager(
			// writeSubset depends on "net/http.(*Transport).roundTrip", it should be ignored without roundTrip
			"net/http.Header.writeSubset",
			"net/http.serverHandler.ServeHTTP",
		)
		assert.Len(t, m.probes, 1)
	})
}

func TestDependencyChecks(t *testing.T) {
	m := fakeManager()

	t.Run("Dependent probes match", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:       "A",
				DependsOn: nil,
			},
			{
				Symbol:       "B",
				DependsOn: []string{"A"},
			},
		}

		assert.NoError(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Second dependent missing", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:       "A",
				DependsOn: nil,
			},
			{
				Symbol:       "B",
				DependsOn: []string{"A", "C"},
			},
		}

		assert.Error(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Second dependent present", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:       "A",
				DependsOn: nil,
			},
			{
				Symbol:       "B",
				DependsOn: []string{"A", "C"},
			},
			{
				Symbol:       "C",
				DependsOn: []string{"A"},
			},
		}

		assert.NoError(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Dependent wrong", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:       "A",
				DependsOn: nil,
			},
			{
				Symbol:       "B",
				DependsOn: []string{"A1"},
			},
		}

		assert.Error(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})

	t.Run("Two probes without dependents", func(t *testing.T) {
		syms := []probe.FunctionSymbol{
			{
				Symbol:       "A",
				DependsOn: nil,
			},
			{
				Symbol:       "B",
				DependsOn: []string{},
			},
		}

		assert.NoError(t, m.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms))
	})
}

func fakeManager(fnNames ...string) *Manager {
	logger := slog.Default()
	probes := []probe.Probe{
		grpcClient.New(logger, ""),
		grpcServer.New(logger, ""),
		httpServer.New(logger, ""),
		httpClient.New(logger, ""),
		dbSql.New(logger, ""),
		kafkaProducer.New(logger, ""),
		kafkaConsumer.New(logger, ""),
		autosdk.New(logger),
		otelTraceGlobal.New(logger),
	}
	ver := semver.New(1, 20, 0, "", "")
	var fn []*binary.Func
	for _, name := range fnNames {
		fn = append(fn, &binary.Func{Name: name})
	}
	m := &Manager{
		logger: slog.Default(),
		cp:     NewNoopConfigProvider(nil),
		probes: make(map[probe.ID]*probeReference),
		proc: &process.Info{
			ID:        1,
			Functions: fn,
			GoVersion: ver,
			Modules:   map[string]*semver.Version{},
		},
	}
	for _, p := range probes {
		m.probes[p.Manifest().ID] = &probeReference{probe: p}
	}
	m.filterUnusedProbes()

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
	bpffsMount = func(info *process.Info) error {
		if info == nil {
			return errors.New("target is nil in Mount")
		}
		return nil
	}
	t.Cleanup(func() { bpffsMount = origBpffsMount })

	origBpffsCleanup := bpffsCleanup
	bpffsCleanup = func(info *process.Info) error {
		if info == nil {
			return errors.New("target is nil in Cleanup")
		}
		return nil
	}
	t.Cleanup(func() { bpffsCleanup = origBpffsCleanup })
}

// noopTraceHandler is a no-op implementation of the [pipeline.Handler]. It is
// used for testing when no telemetry is meant to be recorded.
type noopTraceHandler struct{}

var _ pipeline.TraceHandler = noopTraceHandler{}

// Handle drops the passed telemetry.
func (noopTraceHandler) HandleTrace(pcommon.InstrumentationScope, string, ptrace.SpanSlice) {}

func newNoopHandler() *pipeline.Handler {
	return &pipeline.Handler{TraceHandler: noopTraceHandler{}}
}

func TestRunStoppingByContext(t *testing.T) {
	probeStop := make(chan struct{})
	p := newSlowProbe(probeStop)

	m := &Manager{
		handler: newNoopHandler(),
		logger:  slog.Default(),
		probes:  map[probe.ID]*probeReference{p.Manifest().ID: {probe: p}},
		cp:      NewNoopConfigProvider(nil),
		proc:    new(process.Info),
	}

	mockExeAndBpffs(t)

	ctx, stopCtx := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	err := m.Load(ctx)
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
}

func TestRunStoppingByStop(t *testing.T) {
	p := noopProbe{}

	m := &Manager{
		handler: newNoopHandler(),
		logger:  slog.Default(),
		probes:  map[probe.ID]*probeReference{{}: {probe: &p}},
		cp:      NewNoopConfigProvider(nil),
		proc:    new(process.Info),
	}

	mockExeAndBpffs(t)

	ctx := context.Background()
	errCh := make(chan error, 1)

	err := m.Load(ctx)
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
	assert.NoError(t, err)
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

func (p slowProbe) Run(*pipeline.Handler) {
}

func (p slowProbe) Close() error {
	p.closeSignal <- struct{}{}
	<-p.stop
	return nil
}

func (p slowProbe) Spec() (*ebpf.CollectionSpec, error) {
	return &ebpf.CollectionSpec{
		Maps:     make(map[string]*ebpf.MapSpec),
		Programs: make(map[string]*ebpf.ProgramSpec),
	}, nil
}

func (p slowProbe) InitStartupConfig(*ebpf.Collection, *sampling.Config) (io.Closer, error) {
	return p, nil
}

func (p slowProbe) Manifest() probe.Manifest {
	return probe.Manifest{ID: probe.ID{SpanKind: trace.SpanKindClient, InstrumentedPkg: "slowProbe"}}
}

type noopProbe struct {
	loaded, running, closed atomic.Bool
}

var _ probe.Probe = (*noopProbe)(nil)

func (p *noopProbe) Spec() (*ebpf.CollectionSpec, error) {
	return &ebpf.CollectionSpec{
		Maps:     make(map[string]*ebpf.MapSpec),
		Programs: make(map[string]*ebpf.ProgramSpec),
	}, nil
}

func (p *noopProbe) Manifest() probe.Manifest {
	return probe.Manifest{ID: probe.ID{SpanKind: trace.SpanKindClient, InstrumentedPkg: "noopProbe"}}
}

func (p *noopProbe) InitStartupConfig(*ebpf.Collection, *sampling.Config) (io.Closer, error) {
	p.loaded.Store(true)
	return p, nil
}

func (p *noopProbe) Run(*pipeline.Handler) {
	p.running.Store(true)
}

func (p *noopProbe) Close() error {
	p.closed.Store(true)
	p.loaded.Store(false)
	p.running.Store(false)
	return nil
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
	somePackageProducerProbeID := probe.ID{
		InstrumentedPkg: "some/package",
		SpanKind:        trace.SpanKindProducer,
	}

	netHTTPClientLibID := LibraryID{InstrumentedPkg: "net/http", SpanKind: trace.SpanKindClient}
	netHTTPLibID := LibraryID{InstrumentedPkg: "net/http"}
	falseVal := false

	m := &Manager{
		logger: slog.Default(),
		probes: map[probe.ID]*probeReference{
			netHTTPClientProbeID:       {probe: &noopProbe{}},
			netHTTPServerProbeID:       {probe: &noopProbe{}},
			somePackageProducerProbeID: {probe: &noopProbe{}},
		},
		cp: newDummyProvider(Config{
			InstrumentationLibraryConfigs: map[LibraryID]Library{
				netHTTPClientLibID: {TracesEnabled: &falseVal},
			},
		}),
		proc: new(process.Info),
	}

	mockExeAndBpffs(t)
	runCtx, cancel := context.WithCancel(context.Background())

	err := m.Load(runCtx)
	require.NoError(t, err)

	runErr := make(chan error, 1)

	go func() { runErr <- m.Run(runCtx) }()

	probeRunning := func(id probe.ID) bool {
		p := m.probes[id].probe.(*noopProbe)
		return p.loaded.Load() && p.running.Load()
	}

	probePending := func(id probe.ID) bool {
		p := m.probes[id].probe.(*noopProbe)
		return !p.loaded.Load() && !p.running.Load()
	}

	probeClosed := func(id probe.ID) bool {
		p := m.probes[id].probe.(*noopProbe)
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

func (p *hangingProbe) Run(h *pipeline.Handler) {
	<-p.closeReturned
	// Write after Close has returned.
	h.Trace(ptrace.NewSpanSlice())
}

func (p *hangingProbe) Spec() (*ebpf.CollectionSpec, error) {
	return &ebpf.CollectionSpec{
		Maps:     make(map[string]*ebpf.MapSpec),
		Programs: make(map[string]*ebpf.ProgramSpec),
	}, nil
}

func (p *hangingProbe) Manifest() probe.Manifest {
	return probe.Manifest{ID: probe.ID{SpanKind: trace.SpanKindClient, InstrumentedPkg: "hangingProbe"}}
}

func (p *hangingProbe) InitStartupConfig(*ebpf.Collection, *sampling.Config) (io.Closer, error) {
	return p, nil
}

func (p *hangingProbe) Close() error {
	defer close(p.closeReturned)
	return nil
}

func TestRunStopDeadlock(t *testing.T) {
	// Regression test for #1228.
	p := newHangingProbe()

	m := &Manager{
		handler: newNoopHandler(),
		logger:  slog.Default(),
		probes:  map[probe.ID]*probeReference{{}: {probe: p}},
		cp:      NewNoopConfigProvider(nil),
		proc:    new(process.Info),
	}

	mockExeAndBpffs(t)

	ctx, stopCtx := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	err := m.Load(ctx)
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
}

func TestStopBeforeLoad(t *testing.T) {
	p := noopProbe{}

	m := &Manager{
		handler: newNoopHandler(),
		logger:  slog.Default(),
		probes:  map[probe.ID]*probeReference{{}: {probe: &p}},
		cp:      NewNoopConfigProvider(nil),
		proc:    new(process.Info),
	}

	mockExeAndBpffs(t)
	require.NoError(t, m.Stop())
}

func TestStopBeforeRun(t *testing.T) {
	p := noopProbe{}

	m := &Manager{
		handler: newNoopHandler(),
		logger:  slog.Default(),
		probes:  map[probe.ID]*probeReference{{}: {probe: &p}},
		cp:      NewNoopConfigProvider(nil),
		proc:    new(process.Info),
	}

	mockExeAndBpffs(t)

	err := m.Load(context.Background())
	require.NoError(t, err)
	require.True(t, p.loaded.Load())

	err = m.Stop()
	require.NoError(t, err)
	require.True(t, p.closed.Load())
	require.False(t, p.running.Load())
}
