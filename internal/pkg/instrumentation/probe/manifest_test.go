// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

func fs(s string) FunctionSymbol {
	return FunctionSymbol{Symbol: s, DependsOn: nil}
}

func TestNewManifest(t *testing.T) {
	const (
		spanKind = trace.SpanKindServer
		pkg      = "pkg"

		a = "a"
		b = "b"
		c = "c"
		d = "d"
	)

	var (
		sAAAA = structfield.NewID("a", "a/a", "A", "A")
		sAAAB = structfield.NewID("a", "a/a", "A", "B")
		sAAAC = structfield.NewID("a", "a/a", "A", "C")
		sAABA = structfield.NewID("a", "a/a", "B", "A")
		sAABB = structfield.NewID("a", "a/a", "B", "B")
		sAABC = structfield.NewID("a", "a/a", "B", "C")
		sABAA = structfield.NewID("a", "a/b", "A", "A")
		sBAAA = structfield.NewID("b", "a/a", "A", "A")
	)

	got := NewManifest(
		ID{spanKind, pkg},
		[]structfield.ID{sAABB, sABAA, sAAAA, sAAAC, sBAAA, sAAAB, sAABA, sAABC},
		[]FunctionSymbol{fs(d), fs(a), fs(c), fs(b)},
	)
	want := Manifest{
		ID:           ID{spanKind, pkg},
		StructFields: []structfield.ID{sAAAA, sAAAB, sAAAC, sAABA, sAABB, sAABC, sABAA, sBAAA},
		Symbols:      []FunctionSymbol{fs(a), fs(b), fs(c), fs(d)},
	}
	assert.Equal(t, want, got)
}
