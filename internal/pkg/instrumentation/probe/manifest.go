// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"fmt"
	"sort"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// FunctionSymbol is a function symbol targeted by a uprobe.
type FunctionSymbol struct {
	Symbol    string
	DependsOn []string
}

// Manifest contains information about a package being instrumented.
type Manifest struct {
	// ID is a unique identifier for the probe.
	ID ID

	Consts  []Const
	Uprobes []*Uprobe
}

// StructFields are the struct fields in an instrumented package that are
// used for instrumentation.
func (m Manifest) StructFields() []structfield.ID {
	var structfields []structfield.ID
	for _, cnst := range m.Consts {
		if sfc, ok := cnst.(StructFieldConst); ok {
			structfields = append(structfields, sfc.ID)
		}
		if sfc, ok := cnst.(StructFieldConstMinVersion); ok {
			structfields = append(structfields, sfc.StructField.ID)
		}
	}

	sort.Slice(structfields, func(i, j int) bool {
		if structfields[i].ModPath != structfields[j].ModPath {
			return structfields[i].ModPath < structfields[j].ModPath
		}
		if structfields[i].PkgPath != structfields[j].PkgPath {
			return structfields[i].PkgPath < structfields[j].PkgPath
		}
		if structfields[i].Struct != structfields[j].Struct {
			return structfields[i].Struct < structfields[j].Struct
		}
		return structfields[i].Field < structfields[j].Field
	})

	return structfields
}

// Symbols are the runtime symbols that are used to attach a probe's eBPF
// program to a perf events.
func (m Manifest) Symbols() []FunctionSymbol {
	symbols := make([]FunctionSymbol, 0, len(m.Uprobes))
	for _, up := range m.Uprobes {
		symbols = append(symbols, FunctionSymbol{Symbol: up.Sym, DependsOn: up.DependsOn})
	}
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Symbol < symbols[j].Symbol
	})
	return symbols
}

// ID is a unique identifier for a probe.
type ID struct {
	// SpanKind is the span kind handled by the probe.
	SpanKind trace.SpanKind
	// InstrumentedPkg is the package path of the instrumented code.
	InstrumentedPkg string
}

func (id ID) String() string {
	return fmt.Sprintf("%s/%s", id.InstrumentedPkg, id.SpanKind)
}

// NewManifest returns a new Manifest for the instrumentation probe with name
// that instruments pkg.
func NewManifest(id ID, consts []Const, uprobes []*Uprobe) Manifest {
	return Manifest{
		ID:      id,
		Consts:  consts,
		Uprobes: uprobes,
	}
}
