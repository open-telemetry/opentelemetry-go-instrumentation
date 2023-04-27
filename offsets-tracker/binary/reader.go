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

package binary

import (
	"debug/elf"
	"errors"
	"os"
)

// DataMember defines a data structure.
type DataMember struct {
	StructName string
	Field      string
}

// DataMemberOffset is the offset of a DataMember.
type DataMemberOffset struct {
	*DataMember
	Offset uint64
}

// Results are all the returned offsets for a data member.
type Result struct {
	DataMembers []*DataMemberOffset
}

// ErrOffsetsNotFound is returned when the requested offsets cannot be found.
var ErrOffsetsNotFound = errors.New("could not find offset")

// FindOffsets finds all the dataMembers offsets.
func FindOffsets(file *os.File, dataMembers []*DataMember) (*Result, error) {
	elfF, err := elf.NewFile(file)
	if err != nil {
		return nil, err
	}

	dwarfData, err := elfF.DWARF()
	if err != nil {
		return nil, err
	}

	result := &Result{}
	for _, dm := range dataMembers {
		offset, found := findDataMemberOffset(dwarfData, dm)
		if !found {
			return nil, ErrOffsetsNotFound
		}
		result.DataMembers = append(result.DataMembers, &DataMemberOffset{
			DataMember: dm,
			Offset:     uint64(offset),
		})
	}

	return result, nil
}
