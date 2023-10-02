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
	"io"
	"log"
	"os"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/tools/offsets/binary"
)

// Cache holds already seen offsets.
type Cache struct {
	data *inject.TrackedOffsets
}

// NewCache returns a new [Cache].
func NewCache(prevOffsetFile string) *Cache {
	f, err := os.Open(prevOffsetFile)
	if err != nil {
		fmt.Println("could not find existing offset file, cache will be empty")
		return nil
	}

	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("error reading existing offsets file: %v. Ignoring existing file.\n", err)
		return nil
	}

	var offsets inject.TrackedOffsets
	err = json.Unmarshal(data, &offsets)
	if err != nil {
		log.Printf("error parsing existing offsets file: %v Ignoring existing file.\n", err)
		return nil
	}

	return &Cache{
		data: &offsets,
	}
}

// IsAllInCache checks whether the passed datamembers exist in the cache for a
// given version.
func (c *Cache) IsAllInCache(v *version.Version, dataMembers []*binary.DataMember) ([]*binary.DataMemberOffset, bool) {
	var results []*binary.DataMemberOffset
	for _, dm := range dataMembers {
		off, ok := c.data.GetOffset(dm.StructName, dm.Field, v)
		if !ok {
			return nil, false
		}
		results = append(results, &binary.DataMemberOffset{
			DataMember: dm,
			Offset:     off,
		})
	}
	return results, true
}
