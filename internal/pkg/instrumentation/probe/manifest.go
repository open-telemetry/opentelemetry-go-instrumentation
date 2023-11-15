// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probe

import (
	"sort"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// Manifest contains information about a package being instrumented.
type Manifest struct {
	// Name is the unique name of the instrumentation.
	Name string

	// InstrumentedPkg is the package path of the instrumented code.
	InstrumentedPkg string

	// StructFields are the struct fields in an instrumented package that are
	// used for instrumentation.
	StructFields []structfield.ID

	// Symbols are the runtime symbols that are used to attach a probe's eBPF
	// program to a perf events.
	Symbols []string
}

// NewManifest returns a new Manifest for the instrumentation probe with name
// that instruments pkg. The structfields and symbols will be sorted in-place
// and added directly to the returned Manifest.
func NewManifest(name, pkg string, structfields []structfield.ID, symbols []string) Manifest {
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

	sort.Strings(symbols)

	return Manifest{
		Name:            name,
		InstrumentedPkg: pkg,
		StructFields:    structfields,
		Symbols:         symbols,
	}
}
