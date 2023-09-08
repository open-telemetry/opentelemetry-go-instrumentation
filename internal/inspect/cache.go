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
	"sync"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/auto/internal/pkg/inject"
)

type cache struct {
	log logr.Logger

	dataMu sync.Mutex
	data   *inject.TrackedOffsets
}

func newCache(l logr.Logger, offsetFile string) (*cache, error) {
	c := &cache{log: l.WithName("cache")}

	f, err := os.Open(offsetFile)
	if err != nil {
		return c, err
	}
	defer f.Close()

	c.data = new(inject.TrackedOffsets)
	err = json.NewDecoder(f).Decode(c.data)
	return c, err
}

// Get returns the cached structFieldOffset for the StructField at the
// specified version. If the cache does not contain a valid structFieldOffset
// for the provided values, the returned Offset of the structFieldOffset will
// be -1.
func (c *cache) Get(ver *version.Version, sf StructField) structFieldOffset {
	sfo, ok := c.get(ver, sf)
	msg := "cache "
	if ok {
		msg += "hit"
	} else {
		msg += "miss"
	}
	c.log.V(1).Info(
		msg,
		"version", ver,
		"package", sf.Package,
		"struct", sf.Struct,
		"field", sf.Field,
	)
	return sfo
}

func (c *cache) get(ver *version.Version, sf StructField) (structFieldOffset, bool) {
	sfo := structFieldOffset{StructField: sf, Version: ver, Offset: -1}

	if c.data == nil {
		return sfo, false
	}

	o, ok := c.data.GetOffset(sf.structName(), sf.Field, ver)
	if ok {
		sfo.Offset = int64(o)
	}
	return sfo, ok
}
