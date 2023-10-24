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

package structfield

import "github.com/hashicorp/go-version"

type jsonOffset struct {
	Offset   uint64             `json:"offset"`
	Versions []*version.Version `json:"versions"`
}

type jsonField struct {
	Field   string        `json:"field"`
	Offsets []*jsonOffset `json:"offsets"`
}

func (jf *jsonField) addOffsets(off *Offsets) {
	for o, vers := range off.index() {
		jOff := find(&jf.Offsets, func(jo *jsonOffset) bool {
			return o == jo.Offset
		})
		jOff.Offset = o

		jOff.Versions = mergeSorted(jOff.Versions, vers, func(a, b *version.Version) int {
			return a.Compare(b)
		})
	}
}

type jsonStruct struct {
	Struct string       `json:"struct"`
	Fields []*jsonField `json:"fields"`
}

func (js *jsonStruct) addOffsets(field string, off *Offsets) {
	jf := find(&js.Fields, func(f *jsonField) bool {
		return field == f.Field
	})
	jf.Field = field
	jf.addOffsets(off)
}

type jsonPackage struct {
	Package string        `json:"package"`
	Structs []*jsonStruct `json:"structs"`
}

func (jp *jsonPackage) addOffsets(strct, field string, off *Offsets) {
	js := find(&jp.Structs, func(s *jsonStruct) bool {
		return strct == s.Struct
	})
	js.Struct = strct
	js.addOffsets(field, off)
}

type jsonModule struct {
	Module   string         `json:"module"`
	Packages []*jsonPackage `json:"packages"`
}

func (jm *jsonModule) addOffsets(pkg, strct, field string, off *Offsets) {
	jp := find(&jm.Packages, func(p *jsonPackage) bool {
		return pkg == p.Package
	})
	jp.Package = pkg
	jp.addOffsets(strct, field, off)
}

// find returns the value in slice where f evaluates to true. If none exists a
// new value of *T is created and appended to slice.
func find[T any](slice *[]*T, f func(*T) bool) *T {
	var t *T
	for _, s := range *slice {
		if f(s) {
			t = s
			break
		}
	}
	if t == nil {
		t = new(T)
		*slice = append(*slice, t)
	}
	return t
}

// mergeSorted merges the two sorted slices slice0 and slice1 using the cmp
// function to compare elements.
//
// The cmp function needs to return negative values when a<b, possitive values
// when a>b, and 0 when a==b.
func mergeSorted[T any](slice0, slice1 []T, cmp func(a, b T) int) []T {
	merged := make([]T, 0, len(slice0)+len(slice1))
	i, j := 0, 0

	for i < len(slice0) && j < len(slice1) {
		switch c := cmp(slice0[i], slice1[j]); {
		case c < 0:
			merged = append(merged, slice0[i])
			i++
		case c > 0:
			merged = append(merged, slice1[j])
			j++
		case c == 0:
			merged = append(merged, slice0[i])
			i++
			j++
		}
	}

	// Append any remaining elements from slice0 and slice1.
	for i < len(slice0) {
		merged = append(merged, slice0[i])
		i++
	}
	for j < len(slice1) {
		merged = append(merged, slice1[j])
		j++
	}

	return merged
}
