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

	// StructFields are the struct fields in an instrumented package that are
	// used for instrumentation.
	StructFields []structfield.ID

	// Symbols are the runtime symbols that are used to attach a probe's eBPF
	// program to a perf events.
	Symbols []FunctionSymbol
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
// that instruments pkg. The structfields and symbols will be sorted in-place
// and added directly to the returned Manifest.
func NewManifest(id ID, structfields []structfield.ID, symbols []FunctionSymbol) Manifest {
	sort.Slice(structfields, func(i, j int) bool {
		if structfields[i].ModPath == structfields[j].ModPath {
			if structfields[i].PkgPath == structfields[j].PkgPath {
				if structfields[i].Struct == structfields[j].Struct {
					return structfields[i].Field < structfields[j].Field
				}
				return structfields[i].Struct < structfields[j].Struct
			}
			return structfields[i].PkgPath < structfields[j].PkgPath
		}
		return structfields[i].ModPath < structfields[j].ModPath
	})

	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Symbol < symbols[j].Symbol
	})

	return Manifest{
		ID:           id,
		StructFields: structfields,
		Symbols:      symbols,
	}
}
