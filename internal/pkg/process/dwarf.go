// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// ErrDWARFEntry is returned if an entry is not found within DWARF data.
var ErrDWARFEntry = errors.New("DWARF entry not found")

// DWARF provides convenience in accessing DWARF debugging data.
type DWARF struct {
	Reader *dwarf.Reader
}

// GoStructField returns the offset value of a Go struct field. If the struct
// field cannot be found -1 and a non-nil error will be returned.
func (d DWARF) GoStructField(id structfield.ID) (int64, error) {
	strct := fmt.Sprintf("%s.%s", id.PkgPath, id.Struct)
	if !d.GoToEntry(dwarf.TagStructType, strct) {
		return -1, fmt.Errorf("struct %q not found", strct)
	}

	e, err := d.EntryInChildren(dwarf.TagMember, id.Field)
	if err != nil {
		return -1, fmt.Errorf("struct field %q not found: %w", id.Field, err)
	}

	f, ok := d.Field(e, dwarf.AttrDataMemberLoc)
	if !ok {
		return -1, fmt.Errorf("struct field offset not found: %w", err)
	}

	v, ok := f.Val.(int64)
	if !ok {
		return -1, errors.New("invalid struct field offset")
	}
	return v, nil
}

// GoToEntry reads until the entry with a tag equal to name is found. True is
// returned if the entry is found, otherwise false is returned.
func (d DWARF) GoToEntry(tag dwarf.Tag, name string) bool {
	_, err := d.Entry(tag, name)
	return err == nil
}

// Entry returns the entry with a tag equal to name. ErrDWARFEntry is returned
// if the entry cannot be found.
func (d DWARF) Entry(tag dwarf.Tag, name string) (*dwarf.Entry, error) {
	for {
		entry, err := d.Reader.Next()
		if errors.Is(err, io.EOF) || entry == nil {
			break
		}

		if entry.Tag == tag {
			if f, ok := d.Field(entry, dwarf.AttrName); ok {
				if name == f.Val.(string) {
					return entry, nil
				}
			}
		}
	}
	return nil, ErrDWARFEntry
}

// EntryInChildren returns the entry with a tag equal to name within the
// children of the current entry. ErrDWARFEntry is returned if the entry cannot
// be found.
func (d DWARF) EntryInChildren(tag dwarf.Tag, name string) (*dwarf.Entry, error) {
	for {
		entry, err := d.Reader.Next()
		if errors.Is(err, io.EOF) || entry == nil || entry.Tag == 0 {
			break
		}

		if entry.Tag == tag {
			if f, ok := d.Field(entry, dwarf.AttrName); ok {
				if name == f.Val.(string) {
					return entry, nil
				}
			}
		}
	}
	return nil, ErrDWARFEntry
}

// Field returns the field from the entry e that has attribute a and true.
// If no field is found, an empty field is returned with false.
func (d DWARF) Field(e *dwarf.Entry, a dwarf.Attr) (dwarf.Field, bool) {
	for _, f := range e.Field {
		if f.Attr == a {
			return f, true
		}
	}
	return dwarf.Field{}, false
}
