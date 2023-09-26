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
)

// StructField defines a field of a struct from a package.
type StructField struct {
	// PkgPath is the unique package import path containing the struct.
	PkgPath string
	// Struct is the name of the struct containing the field.
	Struct string
	// Field is the name of the field.
	Field string
}

// structName returns the package path prefixed struct name of s.
func (s StructField) structName() string {
	return fmt.Sprintf("%s.%s", s.PkgPath, s.Struct)
}

// offset returns the field offset found in the DWARF data d and true. If the
// offset is not found in d, 0 and false are returned.
func (s StructField) offset(d *dwarf.Data) (uint64, bool) {
	r := d.Reader()
	if !gotoEntry(r, dwarf.TagStructType, s.structName()) {
		return 0, false
	}

	e, err := findEntry(r, dwarf.TagMember, s.Field)
	if err != nil {
		return 0, false
	}

	f, ok := entryField(e, dwarf.AttrDataMemberLoc)
	if !ok {
		return 0, false
	}

	return uint64(f.Val.(int64)), true
}

// gotoEntry reads from r until the entry with a tag equal to name is found.
// True is returned if the entry is found, otherwise false is returned.
func gotoEntry(r *dwarf.Reader, tag dwarf.Tag, name string) bool {
	_, err := findEntry(r, tag, name)
	return err == nil
}

// findEntry returns the DWARF entry with a tag equal to name read from r. An
// error is returned if the entry cannot be found.
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

// entryField returns the DWARF field from DWARF entry e that has the passed
// DWARF attribute a.
func entryField(e *dwarf.Entry, a dwarf.Attr) (dwarf.Field, bool) {
	for _, f := range e.Field {
		if f.Attr == a {
			return f, true
		}
	}
	return dwarf.Field{}, false
}
