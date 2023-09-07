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

package inspect

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/inject"
)

// StructField defines a field of a struct from package.
type StructField struct {
	Package string
	Struct  string
	Field   string
}

func (s StructField) structName() string {
	return fmt.Sprintf("%s.%s", s.Package, s.Struct)
}

func (s StructField) offset(ver *version.Version, d *dwarf.Data) structFieldOffset {
	sfo := structFieldOffset{
		StructField: s,
		Version:     ver,
		Offset:      -1,
	}

	r := d.Reader()
	if !gotoEntry(r, dwarf.TagStructType, s.structName()) {
		return sfo
	}

	e, err := findEntry(r, dwarf.TagMember, s.Field)
	if err != nil {
		return sfo
	}

	f, ok := entryField(e, dwarf.AttrDataMemberLoc)
	if !ok {
		return sfo
	}
	sfo.Offset = f.Val.(int64)
	return sfo
}

func gotoEntry(r *dwarf.Reader, tag dwarf.Tag, name string) bool {
	_, err := findEntry(r, tag, name)
	return err == nil
}

func findEntry(r *dwarf.Reader, tag dwarf.Tag, name string) (*dwarf.Entry, error) {
	for {
		entry, err := r.Next()
		if err == io.EOF || entry == nil {
			break
		}

		if entry.Tag == tag {
			if f, ok := entryField(entry, dwarf.AttrName); ok {
				if name == f.Val.(string) {
					return entry, nil
				}
			}
		}
	}
	return nil, errors.New("not found")
}

func entryField(e *dwarf.Entry, a dwarf.Attr) (dwarf.Field, bool) {
	for _, f := range e.Field {
		if f.Attr == a {
			return f, true
		}
	}
	return dwarf.Field{}, false
}

type structFieldOffset struct {
	StructField

	Version *version.Version
	Offset  int64
}

func trackedOffsets(results []structFieldOffset) *inject.TrackedOffsets {
	return newTrackedOffsets(indexFields(indexOffsets(results)))
}

func indexOffsets(results []structFieldOffset) map[StructField][]offset {
	offsets := make(map[StructField][]offset)
	for _, r := range results {
		offsets[r.StructField] = append(offsets[r.StructField], offset{
			Value: r.Offset,
			Since: r.Version,
		})
	}
	return offsets
}

func indexFields(offsets map[StructField][]offset) map[StructField][]field {
	fields := make(map[StructField][]field)
	for sf, offs := range offsets {
		r := new(versionRange)
		last := -1
		var collapsed []offset

		sort.Slice(offs, func(i, j int) bool {
			return offs[i].Since.LessThan(offs[j].Since)
		})
		for i, off := range offs {
			if off.Value < 0 {
				if !r.empty() && len(collapsed) > 0 {
					fields[sf] = append(fields[sf], field{
						Vers: r,
						Offs: collapsed,
					})
				}

				r = new(versionRange)
				collapsed = []offset{}
				last = -1
				continue
			}
			r.update(off.Since)

			// Only append if field value changed.
			if last < 0 || off.Value != offs[last].Value {
				collapsed = append(collapsed, off)
			}
			last = i
		}
		if !r.empty() && len(collapsed) > 0 {
			fields[sf] = append(fields[sf], field{
				Vers: r,
				Offs: collapsed,
			})
		}
	}
	return fields
}

func newTrackedOffsets(fields map[StructField][]field) *inject.TrackedOffsets {
	tracked := &inject.TrackedOffsets{
		Data: make(map[string]inject.TrackedStruct),
	}
	for sf, fs := range fields {
		for _, f := range fs {
			key := sf.structName()
			strct, ok := tracked.Data[key]
			if !ok {
				strct = make(inject.TrackedStruct)
				tracked.Data[key] = strct
			}

			strct[sf.Field] = append(strct[sf.Field], f.trackedField())
		}
	}

	return tracked
}

type field struct {
	Vers *versionRange
	Offs []offset
}

func (f field) trackedField() inject.TrackedField {
	vo := make([]inject.VersionedOffset, len(f.Offs))
	for i := range vo {
		vo[i] = f.Offs[i].versionedOffset()
	}

	return inject.TrackedField{
		Versions: f.Vers.versionInfo(),
		Offsets:  vo,
	}
}

type versionRange struct {
	Oldest, Newest *version.Version
}

func (r *versionRange) versionInfo() inject.VersionInfo {
	return inject.VersionInfo{
		Oldest: r.Oldest,
		Newest: r.Newest,
	}
}

func (r *versionRange) empty() bool {
	return r == nil || (r.Oldest == nil && r.Newest == nil)
}

func (r *versionRange) update(ver *version.Version) {
	if r.Oldest == nil || ver.LessThan(r.Oldest) {
		r.Oldest = ver
	}
	if r.Newest == nil || ver.GreaterThan(r.Newest) {
		r.Newest = ver
	}
}

type offset struct {
	Value int64
	Since *version.Version
}

func (o offset) versionedOffset() inject.VersionedOffset {
	return inject.VersionedOffset{
		Offset: uintptr(o.Value),
		Since:  o.Since,
	}
}
