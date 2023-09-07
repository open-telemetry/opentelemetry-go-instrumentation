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
	"os"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/inspect/schema"
)

// Cache holds already seen offsets.
type Cache struct {
	log logr.Logger

	data *schema.TrackedOffsets
}

// New returns an empty, ready to use, [Cache].
func New(l logr.Logger) *Cache {
	return &Cache{
		log:  l.WithName("cache"),
		data: &schema.TrackedOffsets{},
	}
}

// Load returns a new [Cache].
func Load(l logr.Logger, prevOffsetFile string) *Cache {
	f, err := os.Open(prevOffsetFile)
	if err != nil {
		l.Error(err, "could not find existing offset file, cache will be empty")
		return nil
	}

	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		l.Error(err, "failed to read existing offsets")
		return nil
	}

	var offsets schema.TrackedOffsets
	err = json.Unmarshal(data, &offsets)
	if err != nil {
		l.Error(err, "failed to parse existing offsets")
		return nil
	}

	return &Cache{
		log:  l,
		data: &offsets,
	}
}

func (c *Cache) Get(ver *version.Version, pkg, strct, field string) (int64, bool) {
	n, ok := c.get(ver, pkg, strct, field)
	if !ok {
		c.log.Info(
			"cache miss",
			"package", pkg,
			"version", ver,
			"struct", strct,
			"field", field,
		)
	} else {
		c.log.Info(
			"cache hit",
			"package", pkg,
			"version", ver,
			"struct", strct,
			"field", field,
		)
	}
	return n, ok
}

func (c *Cache) get(ver *version.Version, pkg, strct, field string) (int64, bool) {
	name := fmt.Sprintf("%s.%s", pkg, strct)
	ts, ok := c.data.Data[name]
	if !ok {
		return -1, false
	}
	tf, ok := ts[field]
	if !ok {
		return -1, false
	}

	for _, f := range tf {
		if ver.LessThan(f.Versions.Oldest) || ver.GreaterThan(f.Versions.Newest) {
			continue
		}

		off, ok := searchOffset(f, ver)
		if !ok {
			continue
		}
		return off, true
	}
	return -1, false
}

// searchOffset searches an offset from the newest field whose version
// is lower than or equal to the target version.
func searchOffset(field schema.TrackedField, target *version.Version) (int64, bool) {
	// Search from the newest version
	for o := len(field.Offsets) - 1; o >= 0; o-- {
		od := &field.Offsets[o]
		if target.Compare(od.Since) >= 0 {
			// if target version is larger or equal than lib version:
			// we certainly know that it is the most recent tracked offset
			return int64(od.Offset), true
		}
	}

	return 0, false
}
