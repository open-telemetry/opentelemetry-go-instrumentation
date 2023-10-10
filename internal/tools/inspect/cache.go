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

package inspect

import (
	"encoding/json"
	"os"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/inject"
)

// Cache is a cache of struct field offsets.
type Cache struct {
	log  logr.Logger
	data *inject.TrackedOffsets
}

// NewCache loads struct field offsets from offsetFile and returns them as a
// new Cache.
func NewCache(l logr.Logger, offsetFile string) (*Cache, error) {
	c := newCache(l)

	f, err := os.Open(offsetFile)
	if err != nil {
		return c, err
	}
	defer f.Close()

	c.data = new(inject.TrackedOffsets)
	err = json.NewDecoder(f).Decode(c.data)
	return c, err
}

func newCache(l logr.Logger) *Cache {
	return &Cache{log: l.WithName("cache")}
}

// GetOffset returns the cached offset and true for the StructField at the
// specified version. If the cache does not contain a valid offset for the
// provided values, 0 and false are returned.
func (c *Cache) GetOffset(ver *version.Version, sf StructField) (uint64, bool) {
	if c.data == nil {
		return 0, false
	}

	off, ok := c.data.GetOffset(sf.structName(), sf.Field, ver)
	msg := "cache "
	if ok {
		msg += "hit"
	} else {
		msg += "miss"
	}
	c.log.V(1).Info(
		msg,
		"version", ver,
		"package", sf.PkgPath,
		"struct", sf.Struct,
		"field", sf.Field,
	)
	return off, ok
}
