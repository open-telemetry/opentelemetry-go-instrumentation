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

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
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

	m, err := NewManager(logger, nil, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, m)

	return m
}

func TestRunStopping(t *testing.T) {
	probeStop := make(chan struct{})
	p := newSlowProbe(probeStop)

	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")

	m := &Manager{
		logger: logger.WithName("Manager"),
		probes: map[probe.ID]probe.Probe{{}: p},
	}

	origOpenExecutable := openExecutable
	openExecutable = func(string) (*link.Executable, error) { return nil, nil }
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

	var err error
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

func (p slowProbe) Load(*link.Executable, *process.TargetDetails) error {
	return nil
}

func (p slowProbe) Run(c chan<- *probe.Event) {
}

func (p slowProbe) Close() error {
	p.closeSignal <- struct{}{}
	<-p.stop
	return nil
}
