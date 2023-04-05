package writer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/keyval-dev/offsets-tracker/schema"
	"github.com/keyval-dev/offsets-tracker/target"
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
