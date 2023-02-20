package writer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/keyval-dev/offsets-tracker/schema"
	"github.com/keyval-dev/offsets-tracker/target"
	"io/fs"
	"os"
	"strings"
)

func WriteResults(fileName string, results ...*target.Result) error {
	var offsets schema.TrackedOffsets
	for _, r := range results {
		offsets.Data = append(offsets.Data, convertResult(r))
	}

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
