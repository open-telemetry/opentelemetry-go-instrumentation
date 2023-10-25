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

package offsets

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-version"
)

// Index holds all struct field offsets.
type Index struct {
	dataMu sync.RWMutex
	data   map[ID]*Offsets
}

// NewIndex returns a new empty Index.
func NewIndex() *Index {
	return &Index{data: make(map[ID]*Offsets)}
}

// Get returns the Offsets and true for an id contained in the Index i. It will
// return nil and false for any id not contained in i.
func (i *Index) Get(id ID) (*Offsets, bool) {
	i.dataMu.RLock()
	defer i.dataMu.RUnlock()

	return i.get(id)
}

func (i *Index) get(id ID) (*Offsets, bool) {
	o, ok := i.data[id]
	return o, ok
}

// GetOffset returns the offset value and true for the version ver of id
// contained in the Index i. It will return zero and false for any id not
// contained in i.
func (i *Index) GetOffset(id ID, ver *version.Version) (uint64, bool) {
	i.dataMu.RLock()
	defer i.dataMu.RUnlock()

	return i.getOffset(id, ver)
}

func (i *Index) getOffset(id ID, ver *version.Version) (uint64, bool) {
	offs, ok := i.get(id)
	if !ok {
		return 0, false
	}
	o, ok := offs.Get(ver)
	return o, ok
}

// Put stores offsets in the Index i for id.
//
// Any existing offsets stored for id will be replaced. Use PutOffset if you
// would like to update existing offsets for id with an offset value.
func (i *Index) Put(id ID, offsets *Offsets) {
	i.dataMu.Lock()
	defer i.dataMu.Unlock()

	i.put(id, offsets)
}

func (i *Index) put(id ID, offsets *Offsets) {
	i.data[id] = offsets
}

// PutOffset stores the offset value for version ver of id within the Index i.
//
// This will update any existing offsets stored for id with offset. If ver
// already exists within those offsets it will overwrite that value.
func (i *Index) PutOffset(id ID, ver *version.Version, offset uint64) {
	i.dataMu.Lock()
	defer i.dataMu.Unlock()

	i.putOffset(id, ver, offset)
}

func (i *Index) putOffset(id ID, ver *version.Version, offset uint64) {
	off, ok := i.get(id)
	if !ok {
		off = NewOffsets()
		i.put(id, off)
	}
	off.Put(ver, offset)
}

// UnmarshalJSON unmarshals the offset JSON data into i.
func (i *Index) UnmarshalJSON(data []byte) error {
	var pkgs []*jsonPackage
	err := json.Unmarshal(data, &pkgs)
	if err != nil {
		return err
	}

	m := make(map[ID]*Offsets)

	for _, p := range pkgs {
		for _, s := range p.Structs {
			for _, f := range s.Fields {
				for _, o := range f.Offsets {
					for _, v := range o.Versions {
						key := ID{
							PkgPath: p.Package,
							Struct:  s.Struct,
							Field:   f.Field,
						}

						off, ok := m[key]
						if !ok {
							off = new(Offsets)
							m[key] = off
						}
						off.Put(v, o.Offset)
					}
				}
			}
		}
	}

	i.dataMu.Lock()
	i.data = m
	i.dataMu.Unlock()

	return nil
}

// MarshalJSON marshals i into JSON data.
func (i *Index) MarshalJSON() ([]byte, error) {
	i.dataMu.RLock()
	defer i.dataMu.RUnlock()

	var out []*jsonPackage
	for id, off := range i.data {
		jp := find(&out, func(p *jsonPackage) bool {
			return id.PkgPath == p.Package
		})
		jp.Package = id.PkgPath
		jp.addOffsets(id.Struct, id.Field, off)
	}

	// Ensure repeatability by sorting.
	for _, p := range out {
		for _, s := range p.Structs {
			for _, f := range s.Fields {
				sort.Slice(f.Offsets, func(i, j int) bool {
					return f.Offsets[i].Offset < f.Offsets[j].Offset
				})
			}
			sort.Slice(s.Fields, func(i, j int) bool {
				return s.Fields[i].Field < s.Fields[j].Field
			})
		}
		sort.Slice(p.Structs, func(i, j int) bool {
			return p.Structs[i].Struct < p.Structs[j].Struct
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Package < out[j].Package
	})

	return json.Marshal(out)
}

// ID is a struct field identifier for an offset.
type ID struct {
	// PkgPath package import path containing the struct field.
	PkgPath string
	// Struct is the name of the struct containing the field.
	Struct string
	// Field is the field name.
	Field string
}

// NewID returns a new ID using pkg for the PkgPath, strct for the Struct, and
// field for the Field.
func NewID(pkg, strct, field string) ID {
	return ID{PkgPath: pkg, Struct: strct, Field: field}
}

func (i ID) String() string {
	return fmt.Sprintf("%s.%s:%s", i.PkgPath, i.Struct, i.Field)
}

// Offsets are the byte offsets for a struct field at specific versions of the
// package containing struct.
type Offsets struct {
	// mu ensures synchronous access to all Offsets fields.
	mu sync.RWMutex

	// values is a map between version and offset value.
	values map[verKey]offsetVersion
}

// NewOffsets returns a new empty *Offsets.
func NewOffsets() *Offsets {
	return &Offsets{values: make(map[verKey]offsetVersion)}
}

// Get returns the offset in bytes and true if known. Otherwise, 0 and false
// are returned.
func (o *Offsets) Get(ver *version.Version) (uint64, bool) {
	if o == nil {
		return 0, false
	}

	o.mu.RLock()
	v, ok := o.values[newVerKey(ver)]
	o.mu.RUnlock()
	return v.offset, ok
}

// Put sets the offset value for ver. If an offset for ver is already known
// (i.e. ver.Equal(other) == true), this will overwrite that value.
func (o *Offsets) Put(ver *version.Version, offset uint64) {
	ov := offsetVersion{offset: offset, version: ver}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.values == nil {
		o.values = map[verKey]offsetVersion{newVerKey(ver): ov}
		return
	}

	o.values[newVerKey(ver)] = ov
}

func (o *Offsets) index() map[uint64][]*version.Version {
	o.mu.RLock()
	defer o.mu.RUnlock()

	out := make(map[uint64][]*version.Version)
	for _, ov := range o.values {
		vers, ok := out[ov.offset]
		if ok {
			i := sort.Search(len(vers), func(i int) bool {
				return vers[i].GreaterThanOrEqual(ov.version)
			})
			vers = append(vers, nil)
			copy(vers[i+1:], vers[i:])
			vers[i] = ov.version
		} else {
			vers = append(vers, ov.version)
		}
		out[ov.offset] = vers
	}
	return out
}

type verKey struct {
	major, minor, patch uint64
	prerelease          string
	metadata            string
}

func newVerKey(v *version.Version) verKey {
	var segs [3]int
	copy(segs[:], v.Segments())
	return verKey{
		major:      uint64(segs[0]),
		minor:      uint64(segs[1]),
		patch:      uint64(segs[2]),
		prerelease: v.Prerelease(),
		metadata:   v.Metadata(),
	}
}

type offsetVersion struct {
	offset  uint64
	version *version.Version
}

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
