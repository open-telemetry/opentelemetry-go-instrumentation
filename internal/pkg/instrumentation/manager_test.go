//go:build !multi_kernel_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"log"
	"os"
	"testing"

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
