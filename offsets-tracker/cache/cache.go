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

package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"go.opentelemetry.io/auto/offsets-tracker/binary"
	"go.opentelemetry.io/auto/offsets-tracker/schema"
)

// Cache holds already seen offsets.
type Cache struct {
	data *schema.TrackedOffsets
}

// NewCache returns a new [Cache].
func NewCache(prevOffsetFile string) *Cache {
	f, err := os.Open(prevOffsetFile)
	if err != nil {
		fmt.Println("could not find existing offset file, cache will be empty")
		return nil
	}

	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("error reading existing offsets file: %v. Ignoring existing file.\n", err)
		return nil
	}

	var offsets schema.TrackedOffsets
	err = json.Unmarshal(data, &offsets)
	if err != nil {
		log.Printf("error parsing existing offsets file: %v Ignoring existing file.\n", err)
		return nil
	}

	return &Cache{
		data: &offsets,
	}
}

// IsAllInCache returns all DataMemberOffset for a module with modName and
// version and offsets describe by dm.
func (c *Cache) IsAllInCache(modName string, version string, dm []*binary.DataMember) ([]*binary.DataMemberOffset, bool) {
	for _, item := range c.data.Data {
		if item.Name == modName {
			offsetsFound := 0
			var results []*binary.DataMemberOffset
			for _, offsets := range item.DataMembers {
				for _, targetDm := range dm {
					if offsets.Struct == targetDm.StructName && offsets.Field == targetDm.Field {
						for _, ver := range offsets.Offsets {
							if ver.Version == version {
								results = append(results, &binary.DataMemberOffset{
									DataMember: targetDm,
									Offset:     ver.Offset,
								})
								offsetsFound++
							}
						}
					}
				}
			}
			if offsetsFound == len(dm) {
				return results, true
			}
			return nil, false
		}
	}

	return nil, false
}
