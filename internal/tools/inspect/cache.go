// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// Cache is a cache of struct field offsets.
type Cache struct {
	log  *slog.Logger
	data *structfield.Index
}

// NewCache loads struct field offsets from offsetFile and returns them as a
// new Cache.
func NewCache(l *slog.Logger, offsetFile string) (*Cache, error) {
	c := newCache(l)

	f, err := os.Open(offsetFile)
	if err != nil {
		return c, err
	}
	defer f.Close()

	c.data = structfield.NewIndex()
	err = json.NewDecoder(f).Decode(&c.data)
	return c, err
}

func newCache(l *slog.Logger) *Cache {
	return &Cache{log: l}
}

// GetOffset returns the cached offset key and true for the id at the specified
// version is found in the cache. If the cache does not contain a valid offset for the provided
// values, 0 and false are returned.
func (c *Cache) GetOffset(ver *semver.Version, id structfield.ID) (structfield.OffsetKey, bool) {
	if c.data == nil {
		return structfield.OffsetKey{}, false
	}

	off, ok := c.data.GetOffset(id, ver)
	msg := "cache "
	if ok {
		msg += "hit"
	} else {
		msg += "miss"
	}
	c.log.Debug(msg, "version", ver, "id", id)
	return off, ok
}
