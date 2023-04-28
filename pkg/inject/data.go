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

// TrackedOffsets are the offsets for all instrumented packages.
type TrackedOffsets struct {
	// Data key: struct name, which includes the library name in external
	// libraries.
	Data map[string]TrackedStruct `json:"data"`
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
	Oldest string `json:"oldest"`
	Newest string `json:"newest"`
}

// VersionedOffset is the offset for a particular version of a data type from a
// package.
type VersionedOffset struct {
	Offset uint64 `json:"offset"`
	Since  string `json:"since"`
}
