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

package schema

// TrackedOffsets are all the tracked offsets.
type TrackedOffsets struct {
	Data []TrackedLibrary `json:"data"`
}

// TrackedLibrary are all the offsets for a tracked package.
type TrackedLibrary struct {
	Name        string              `json:"name"`
	DataMembers []TrackedDataMember `json:"data_members"`
}

// TrackedDataMember are the Offsets for struct from a package.
type TrackedDataMember struct {
	Struct  string            `json:"struct"`
	Field   string            `json:"field_name"`
	Offsets []VersionedOffset `json:"offsets"`
}

// VersionedOffset if a offset for a version of a module.
type VersionedOffset struct {
	Offset  uint64 `json:"offset"`
	Version string `json:"version"`
}
