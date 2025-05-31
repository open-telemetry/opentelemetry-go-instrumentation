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

	consts := []Const{
		StructFieldConst{
			Key: "saaaa",
			ID:  structfield.NewID("a", "a/a", "A", "A"),
		},
		StructFieldConst{
			Key: "saaab",
			ID:  structfield.NewID("a", "a/a", "A", "B"),
		},
		StructFieldConst{
			Key: "saaac",
			ID:  structfield.NewID("a", "a/a", "A", "C"),
		},
		StructFieldConst{
			Key: "saaba",
			ID:  structfield.NewID("a", "a/a", "B", "A"),
		},
		StructFieldConst{
			Key: "saabb",
			ID:  structfield.NewID("a", "a/a", "B", "B"),
		},
		StructFieldConst{
			Key: "saabc",
			ID:  structfield.NewID("a", "a/a", "B", "C"),
		},
		StructFieldConst{
			Key: "sabaa",
			ID:  structfield.NewID("a", "a/b", "A", "A"),
		},
		StructFieldConst{
			Key: "sbaaa",
			ID:  structfield.NewID("b", "a/a", "A", "A"),
		},
	}

	uprobes := []*Uprobe{
		{Sym: d},
		{Sym: a},
		{Sym: c},
		{Sym: b},
	}

	got := NewManifest(ID{spanKind, pkg}, consts, uprobes)
	want := Manifest{
		ID:      ID{spanKind, pkg},
		Consts:  consts,
		Uprobes: uprobes,
	}
	assert.Equal(t, want, got)

	expectedStructFields := []structfield.ID{sAAAA, sAAAB, sAAAC, sAABA, sAABB, sAABC, sABAA, sBAAA}
	assert.Equal(t, expectedStructFields, got.StructFields())

	expectedSymbols := []FunctionSymbol{fs(a), fs(b), fs(c), fs(d)}
	assert.Equal(t, expectedSymbols, got.Symbols())
}
