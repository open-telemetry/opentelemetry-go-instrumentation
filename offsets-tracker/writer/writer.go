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

package writer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"go.opentelemetry.io/auto/offsets-tracker/schema"
	"go.opentelemetry.io/auto/offsets-tracker/target"
)

func WriteResults(fileName string, results ...*target.Result) error {
	var offsets schema.TrackedOffsets
	for _, r := range results {
		offsets.Data = append(offsets.Data, convertResult(r))
	}

	// sort data for consistent output
	for i := 0; i < len(offsets.Data); i++ {
		trackedLibrary := offsets.Data[i]
		sort.Slice(trackedLibrary.DataMembers, func(i, j int) bool {
			dataMemberi := trackedLibrary.DataMembers[i]
			dataMemberj := trackedLibrary.DataMembers[j]
			if dataMemberi.Struct != dataMemberj.Struct {
				return dataMemberi.Struct < dataMemberj.Struct
			}
			return dataMemberi.Field < dataMemberj.Field
		})
	}
	sort.Slice(offsets.Data, func(i, j int) bool {
		trackedLibraryi := offsets.Data[i]
		trackedLibraryj := offsets.Data[j]
		return trackedLibraryi.Name < trackedLibraryj.Name
	})

	jsonData, err := json.Marshal(&offsets)
	if err != nil {
		return err
	}

	var prettyJson bytes.Buffer
	err = json.Indent(&prettyJson, jsonData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, prettyJson.Bytes(), fs.ModePerm)
}

func convertResult(r *target.Result) schema.TrackedLibrary {
	tl := schema.TrackedLibrary{
		Name: r.ModuleName,
	}

	offsetsMap := make(map[string][]schema.VersionedOffset)
	for _, vr := range r.ResultsByVersion {
		for _, od := range vr.OffsetData.DataMembers {
			key := fmt.Sprintf("%s,%s", od.StructName, od.Field)
			offsetsMap[key] = append(offsetsMap[key], schema.VersionedOffset{
				Offset:  od.Offset,
				Version: vr.Version,
			})
		}
	}

	for key, offsets := range offsetsMap {
		parts := strings.Split(key, ",")
		tl.DataMembers = append(tl.DataMembers, schema.TrackedDataMember{
			Struct:  parts[0],
			Field:   parts[1],
			Offsets: offsets,
		})
	}

	return tl
}
