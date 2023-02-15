package cache

import (
	"encoding/json"
	"fmt"
	"github.com/keyval-dev/offsets-tracker/binary"
	"github.com/keyval-dev/offsets-tracker/schema"
	"io/ioutil"
	"log"
	"os"
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
			} else {
				return nil, false
			}
		}
	}

	return nil, false
}
