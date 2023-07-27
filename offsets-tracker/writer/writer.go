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

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/offsets-tracker/schema"
	"go.opentelemetry.io/auto/offsets-tracker/target"
	"go.opentelemetry.io/auto/offsets-tracker/versions"
)

// WriteResults writes results to fileName.
func WriteResults(fileName string, results ...*target.Result) error {
	offsets := schema.TrackedOffsets{
		Data: map[string]schema.TrackedStruct{},
	}
	for _, r := range results {
		convertResult(r, &offsets)
	}

	jsonData, err := json.Marshal(&offsets)
	if err != nil {
		return err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, prettyJSON.Bytes(), fs.ModePerm)
}

func convertResult(r *target.Result, offsets *schema.TrackedOffsets) {
	offsetsMap := make(map[string][]schema.VersionedOffset)
	for _, vr := range r.ResultsByVersion {
		for _, od := range vr.OffsetData.DataMembers {
			key := fmt.Sprintf("%s,%s", od.StructName, od.Field)
			offsetsMap[key] = append(offsetsMap[key], schema.VersionedOffset{
				Offset: od.Offset,
				Since:  vr.Version,
			})
		}
	}

	// normalize offsets: just annotate the offsets from the version
	// that changed them
	fieldVersionsMap := map[string]hiLoSemVers{}
	for key, offs := range offsetsMap {
		if len(offs) == 0 {
			continue
		}
		// the algorithm below assumes offsets versions are sorted from older to newer
		sort.Slice(offs, func(i, j int) bool {
			return versions.MustParse(offs[i].Since).
				LessThanOrEqual(versions.MustParse(offs[j].Since))
		})

		hilo := hiLoSemVers{}
		var om []schema.VersionedOffset
		var last schema.VersionedOffset
		for n, off := range offs {
			hilo.updateModuleVersion(off.Since)
			// only append versions that changed the field value from its predecessor
			if n == 0 || off.Offset != last.Offset {
				om = append(om, off)
			}
			last = off
		}
		offsetsMap[key] = om
		fieldVersionsMap[key] = hilo
	}

	// Append offsets as fields to the existing file map map
	for key, offs := range offsetsMap {
		parts := strings.Split(key, ",")
		strFields, ok := offsets.Data[parts[0]]
		if !ok {
			strFields = schema.TrackedStruct{}
			offsets.Data[parts[0]] = strFields
		}
		hl := fieldVersionsMap[key]
		strFields[parts[1]] = schema.TrackedField{
			Offsets: offs,
			Versions: schema.VersionInfo{
				Oldest: hl.lo.String(),
				Newest: hl.hi.String(),
			},
		}
	}
}

// hiLoSemVers track highest and lowest version.
type hiLoSemVers struct {
	hi *version.Version
	lo *version.Version
}

func (hl *hiLoSemVers) updateModuleVersion(vr string) {
	ver := versions.MustParse(vr)

	if hl.lo == nil || ver.LessThan(hl.lo) {
		hl.lo = ver
	}
	if hl.hi == nil || ver.GreaterThan(hl.hi) {
		hl.hi = ver
	}
}
