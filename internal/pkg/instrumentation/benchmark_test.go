//go:build !ebpf_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func benchmarkFilterUnusedProbes(b *testing.B, count int) {
	fnNames := make([]string, count)
	for i := 0; i < count; i++ {
		fnNames[i] = strconv.Itoa(i)
	}

	manager := fakeManager(fnNames...)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		manager.filterUnusedProbes()
	}
}

func BenchmarkFilterUnusedProbes10(b *testing.B) {
	benchmarkFilterUnusedProbes(b, 10)
}

func BenchmarkFilterUnusedProbes100(b *testing.B) {
	benchmarkFilterUnusedProbes(b, 100)
}

func BenchmarkFilterUnusedProbes1000(b *testing.B) {
	benchmarkFilterUnusedProbes(b, 1000)
}

func benchmarkValidateProbeDependents(b *testing.B, count int) {
	manager := &Manager{}

	syms := make([]probe.FunctionSymbol, count)

	for i := 0; i < count; i++ {
		syms[i] = probe.FunctionSymbol{
			Symbol:    strconv.Itoa(i),
			DependsOn: nil,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := manager.validateProbeDependents(probe.ID{InstrumentedPkg: "test"}, syms)
		require.NoError(b, err)
	}
}

func BenchmarkValidateProbeDependents10(t *testing.B) {
	benchmarkValidateProbeDependents(t, 10)
}

func BenchmarkValidateProbeDependents100(b *testing.B) {
	benchmarkValidateProbeDependents(b, 100)
}

func BenchmarkValidateProbeDependents1000(b *testing.B) {
	benchmarkValidateProbeDependents(b, 1000)
}
