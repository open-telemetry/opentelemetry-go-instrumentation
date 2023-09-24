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

package inject

import "github.com/hashicorp/go-version"

// TrackedOffsets are the offsets for all instrumented packages.
type TrackedOffsets struct {
	// Data key: struct name, which includes the library name in external
	// libraries.
	Data map[string]TrackedStruct `json:"data"`
}

// GetOffset returns the struct field offset for at the specified version ver
// and true if o contains that offset. A value of 0 and false will be returned
// if o does not contain the offset.
func (o *TrackedOffsets) GetOffset(strct, field string, ver *version.Version) (uint64, bool) {
	sMap, ok := o.Data[strct]
	if !ok {
		return 0, false
	}

	f, ok := sMap[field]
	if !ok {
		return 0, false
	}

	if ver.LessThan(f.Versions.Oldest) || ver.GreaterThan(f.Versions.Newest) {
		return 0, false
	}

	// Search from the newest version (last in the slice).
	for o := len(f.Offsets) - 1; o >= 0; o-- {
		od := &f.Offsets[o]
		if ver.GreaterThanOrEqual(od.Since) {
			return od.Offset, true
		}
	}

	return 0, false
}

// TrackedStruct maps fields names to the tracked fields offsets.
type TrackedStruct map[string]TrackedField

// TrackedField are the field offsets for a tracked struct.
type TrackedField struct {
	// Versions range that are tracked for this given field
	Versions VersionInfo `json:"versions"`
	// Offsets are the sorted version offsets for the field. These need to be
	// sorted in descending order.
	Offsets []VersionedOffset `json:"offsets"`
}

// VersionInfo is the span of supported versions.
type VersionInfo struct {
	Oldest *version.Version `json:"oldest"`
	Newest *version.Version `json:"newest"`
}

// VersionedOffset is the offset for a particular version of a data type from a
// package.
type VersionedOffset struct {
	Offset uint64           `json:"offset"`
	Since  *version.Version `json:"since"`
}
