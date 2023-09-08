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

func (o *TrackedOffsets) GetOffset(strct, field string, ver *version.Version) (uintptr, bool) {
	sMap, ok := o.Data[strct]
	if !ok {
		return 0, false
	}
	fMap, ok := sMap[field]
	if !ok {
		return 0, false
	}
	for _, field := range fMap {
		if ver.LessThan(field.Versions.Oldest) || ver.GreaterThan(field.Versions.Newest) {
			continue
		}
		// Search from the newest version (last in the slice)
		for o := len(field.Offsets) - 1; o >= 0; o-- {
			od := &field.Offsets[o]
			if ver.GreaterThanOrEqual(od.Since) {
				return od.Offset, true
			}
		}
	}

	return 0, false
}

// TrackedStruct maps fields names to the tracked fields offsets.
type TrackedStruct map[string][]TrackedField

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
	Offset uintptr          `json:"offset"`
	Since  *version.Version `json:"since"`
}
