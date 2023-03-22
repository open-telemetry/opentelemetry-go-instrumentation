package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/hashicorp/go-version"

	"github.com/open-telemetry/offsets-tracker/binary"
	"github.com/open-telemetry/offsets-tracker/schema"
	"github.com/open-telemetry/offsets-tracker/versions"
)

type Cache struct {
	data *schema.TrackedOffsets
}

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

// IsAllInCache checks whether the passed datamembers exist in the cache for a given version
func (c *Cache) IsAllInCache(version string, dataMembers []*binary.DataMember) ([]*binary.DataMemberOffset, bool) {
	var results []*binary.DataMemberOffset
	for _, dm := range dataMembers {
		// first, look for the field and check that the target version is in chache
		strct, ok := c.data.Data[dm.StructName]
		if !ok {
			return nil, false
		}
		field, ok := strct[dm.Field]
		if !ok {
			return nil, false
		}
		if !versions.Between(version, field.Versions.Oldest, field.Versions.Newest) {
			return nil, false
		}

		off, ok := searchOffset(field, version)
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

// searchOffset searches an offset from the newest field whose version
// is lower than or equal to the target version
func searchOffset(field schema.TrackedField, targetVersion string) (uint64, bool) {
	target := versions.MustParse(targetVersion)

	// Search from the newest version
	for o := len(field.Offsets) - 1; o >= 0; o-- {
		od := &field.Offsets[o]
		fieldVersion, err := version.NewVersion(od.Since)
		if err != nil {
			// Malformed version: return not found
			return 0, false
		}
		if target.Compare(fieldVersion) >= 0 {
			// if target version is larger or equal than lib version:
			// we certainly know that it is the most recent tracked offset
			return od.Offset, true
		}
	}

	return 0, false
}
