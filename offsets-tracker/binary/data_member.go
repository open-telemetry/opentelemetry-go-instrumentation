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
	"debug/dwarf"
	"io"
)

func findDataMemberOffset(dwarfData *dwarf.Data, dm *DataMember) (int64, bool) {
	reader := dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err == io.EOF || entry == nil {
			break
		}

		if entry.Tag == dwarf.TagStructType {
			// Go through fields
			for _, field := range entry.Field {
				if field.Attr == dwarf.AttrName {
					str := field.Val.(string)
					if str == dm.StructName {
						return findFieldOffset(reader, dm.Field)
					}
				}
			}
		}
	}

	return 0, false
}

func findFieldOffset(reader *dwarf.Reader, field string) (int64, bool) {
	for {
		entry, err := reader.Next()
		if err == io.EOF || entry == nil {
			break
		}

		for _, f := range entry.Field {
			if f.Attr == dwarf.AttrName {
				str := f.Val.(string)
				if str == field {
					return findOffsetByEntry(entry)
				}
			}
		}
	}

	return 0, false
}

func findOffsetByEntry(entry *dwarf.Entry) (int64, bool) {
	for _, field := range entry.Field {
		if field.Attr == dwarf.AttrDataMemberLoc {
			return field.Val.(int64), true
		}
	}

	return 0, false
}
