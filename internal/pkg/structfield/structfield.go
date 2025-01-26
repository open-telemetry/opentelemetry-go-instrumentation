// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package structfield provides types to track struct field offsets.
package structfield

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
func (i *Index) GetOffset(id ID, ver *version.Version) (OffsetKey, bool) {
	i.dataMu.RLock()
	defer i.dataMu.RUnlock()

	return i.getOffset(id, ver)
}

// GetLatestOffset returns the latest known offset value and version for id
// contained in the Index i.
func (i *Index) GetLatestOffset(id ID) (OffsetKey, *version.Version) {
	i.dataMu.RLock()
	defer i.dataMu.RUnlock()

	offs, ok := i.get(id)
	if !ok {
		return OffsetKey{}, nil
	}
	off, ver := offs.getLatest()
	return off, ver.ToVersion()
}

func (i *Index) getOffset(id ID, ver *version.Version) (OffsetKey, bool) {
	offs, ok := i.get(id)
	if !ok {
		return OffsetKey{}, false
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
func (i *Index) PutOffset(id ID, ver *version.Version, offset uint64, valid bool) {
	i.dataMu.Lock()
	defer i.dataMu.Unlock()

	i.putOffset(id, ver, offset, valid)
}

func (i *Index) putOffset(id ID, ver *version.Version, offset uint64, valid bool) {
	off, ok := i.get(id)
	if !ok {
		off = NewOffsets()
		i.put(id, off)
	}
	off.Put(ver, OffsetKey{Offset: offset, Valid: valid})
}

// UnmarshalJSON unmarshals the offset JSON data into i.
func (i *Index) UnmarshalJSON(data []byte) error {
	var mods []*jsonModule
	err := json.Unmarshal(data, &mods)
	if err != nil {
		return err
	}

	m := make(map[ID]*Offsets)

	for _, mod := range mods {
		for _, p := range mod.Packages {
			for _, s := range p.Structs {
				for _, f := range s.Fields {
					for _, o := range f.Offsets {
						for _, v := range o.Versions {
							key := ID{
								ModPath: mod.Module,
								PkgPath: p.Package,
								Struct:  s.Struct,
								Field:   f.Field,
							}

							off, ok := m[key]
							if !ok {
								off = new(Offsets)
								m[key] = off
							}

							if o.Offset == nil {
								off.Put(v, OffsetKey{Valid: false})
							} else {
								off.Put(v, OffsetKey{Offset: *o.Offset, Valid: true})
							}
						}
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

	var out []*jsonModule
	for id, off := range i.data {
		jm := find(&out, func(p *jsonModule) bool {
			return id.ModPath == p.Module
		})
		jm.Module = id.ModPath
		jm.addOffsets(id.PkgPath, id.Struct, id.Field, off)
	}

	// Ensure repeatability by sorting.
	for _, m := range out {
		for _, p := range m.Packages {
			for _, s := range p.Structs {
				for _, f := range s.Fields {
					sort.Slice(f.Offsets, func(i, j int) bool {
						if f.Offsets[i].Offset == nil {
							return true
						}
						if f.Offsets[j].Offset == nil {
							return false
						}
						return *f.Offsets[i].Offset < *f.Offsets[j].Offset
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
		sort.Slice(m.Packages, func(i, j int) bool {
			return m.Packages[i].Package < m.Packages[j].Package
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Module < out[j].Module
	})

	return json.Marshal(out)
}

// ID is a struct field identifier for an offset.
type ID struct {
	// ModPath is the module path containing the struct field package.
	//
	// If set to "std", the struct field belongs to the standard Go library.
	ModPath string
	// PkgPath package import path containing the struct field.
	PkgPath string
	// Struct is the name of the struct containing the field.
	Struct string
	// Field is the field name.
	Field string
}

// NewID returns a new ID using pkg for the PkgPath, strct for the Struct, and
// field for the Field.
func NewID(mod, pkg, strct, field string) ID {
	return ID{ModPath: mod, PkgPath: pkg, Struct: strct, Field: field}
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

	// uo is the single offset in the values map.
	// If there is only one offset, this will be that offset and valid will be true.
	// Otherwise, valid is false
	uo uniqueOffset
}

type uniqueOffset struct {
	value uint64
	valid bool
}

// NewOffsets returns a new empty *Offsets.
func NewOffsets() *Offsets {
	return &Offsets{values: make(map[verKey]offsetVersion)}
}

// Get returns the offset in bytes and true if known. Otherwise, 0 and false
// are returned.
func (o *Offsets) Get(ver *version.Version) (OffsetKey, bool) {
	if o == nil {
		return OffsetKey{}, false
	}

	o.mu.RLock()
	v, ok := o.values[newVerKey(ver)]
	o.mu.RUnlock()

	if strings.HasPrefix(ver.String(), "0.0.0") && !ok && o.uo.valid {
		// If we don't have the exact version, but we only have one offset, we
		// fallback to use that offset. This can happen when a non official version is being used
		// which contains commit hash in the version string.
		return OffsetKey{Offset: o.uo.value, Valid: true}, true
	}

	return v.offset, ok
}

// getLatest returns the latest known offset value and version.
func (o *Offsets) getLatest() (OffsetKey, verKey) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	latestVersion := verKey{}
	val := OffsetKey{}
	for verKey, ov := range o.values {
		if verKey.GreaterThan(latestVersion) && ov.offset.Valid {
			latestVersion = verKey
			val = ov.offset
		}
	}

	return val, latestVersion
}

// Put sets the offset value for ver. If an offset for ver is already known
// (i.e. ver.Equal(other) == true), this will overwrite that value.
func (o *Offsets) Put(ver *version.Version, offset OffsetKey) {
	ov := offsetVersion{offset: offset, version: ver}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.values == nil {
		o.values = map[verKey]offsetVersion{newVerKey(ver): ov}
		o.uo.valid = ov.offset.Valid
		o.uo.value = ov.offset.Offset
		return
	}

	o.values[newVerKey(ver)] = ov

	if o.uo.valid && o.uo.value != ov.offset.Offset {
		o.uo.valid = false
	}
}

func (v verKey) GreaterThan(other verKey) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	if v.minor != other.minor {
		return v.minor > other.minor
	}
	if v.patch != other.patch {
		return v.patch > other.patch
	}
	return false
}

func (v verKey) ToVersion() *version.Version {
	vs := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.prerelease != "" {
		vs += "-" + v.prerelease
	}
	ver, _ := version.NewVersion(vs)
	return ver
}

func (o *Offsets) index() map[OffsetKey][]*version.Version {
	o.mu.RLock()
	defer o.mu.RUnlock()

	out := make(map[OffsetKey][]*version.Version)
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

// OffsetKey is the offset of a specific struct field in a specific version.
// If Valid is false, the offset is not known for the struct field at the
// specified version.
type OffsetKey struct {
	Offset uint64
	Valid  bool
}

func newVerKey(v *version.Version) verKey {
	var segs [3]int
	copy(segs[:], v.Segments())
	return verKey{
		major:      uint64(max(segs[0], 0)), // nolint: gosec  // Bounded.
		minor:      uint64(max(segs[1], 0)), // nolint: gosec  // Bounded.
		patch:      uint64(max(segs[2], 0)), // nolint: gosec  // Bounded.
		prerelease: v.Prerelease(),
		metadata:   v.Metadata(),
	}
}

type offsetVersion struct {
	offset  OffsetKey
	version *version.Version
}
