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
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

func TestNewManifest(t *testing.T) {
	const (
		name = "name"
		pkg  = "pkg"

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
		name,
		pkg,
		[]structfield.ID{sAABB, sABAA, sAAAA, sAAAC, sBAAA, sAAAB, sAABA, sAABC},
		[]string{d, a, c, b},
	)
	want := Manifest{
		Name:            name,
		InstrumentedPkg: pkg,
		StructFields:    []structfield.ID{sAAAA, sAAAB, sAAAC, sAABA, sAABB, sAABC, sABAA, sBAAA},
		Symbols:         []string{a, b, c, d},
	}
	assert.Equal(t, want, got)
}
