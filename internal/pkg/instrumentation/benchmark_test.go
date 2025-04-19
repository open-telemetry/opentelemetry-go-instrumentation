// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"strconv"
	"testing"
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

func Benchmark_validateProbeDependents10(t *testing.B) {
	benchmarkValidateProbeDependents(t, 10)
}

func Benchmark_validateProbeDependents100(b *testing.B) {
	benchmarkValidateProbeDependents(b, 100)
}

func Benchmark_validateProbeDependents1000(b *testing.B) {
	benchmarkValidateProbeDependents(b, 1000)
}
