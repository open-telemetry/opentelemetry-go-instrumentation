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
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/auto/internal/inspect/cache"
	"go.opentelemetry.io/auto/internal/inspect/schema"
	"go.opentelemetry.io/auto/internal/inspect/versions"
	"golang.org/x/sync/errgroup"
)

const (
	defaultStoragePath = "./.offset-tracker"

	// TODO: minGoVersion     = "1.12"
	minGoVersion     = "1.20"
	defaultGoVersion = "1.21.0"

	defaultNWorkers = 20
)

// goVersions are the versions of Go supported.
var goVersions []*version.Version

func init() {
	var err error
	goVersions, err = versions.Go(">= " + minGoVersion)
	if err != nil {
		fmt.Printf("failed to get Go versions: %v", err)
		os.Exit(1)
	}
}

type manifest struct {
	Renderer Renderer
	// TODO: replace with appBuilder func() (string, error)
	GoVer  string
	GoPath string
	AppVer *version.Version
	Fields []StructField
}

type Inspector struct {
	NWorkers int

	log     logr.Logger
	cache   *cache.Cache
	storage *storage
	builder *builder

	manifests []manifest
}

func New(l logr.Logger, c *cache.Cache, storage string) (*Inspector, error) {
	if c == nil {
		c = cache.New(l)
	}

	if storage == "" {
		storage = defaultStoragePath
	}

	s, err := newStorage(l, storage)
	if err != nil {
		return nil, err
	}

	return &Inspector{
		NWorkers: defaultNWorkers,
		log:      l,
		cache:    c,
		storage:  s,
		builder:  newBuilder(l),
	}, nil
}

func (i *Inspector) Inspect3rdParty(r Renderer, vFn func() ([]*version.Version, error), fields []StructField) error {
	vers, err := vFn()
	if err != nil {
		return err
	}

	goPath, err := i.storage.getGo(defaultGoVersion)
	if err != nil {
		return err
	}

	for _, v := range vers {
		i.manifests = append(i.manifests, manifest{
			Renderer: r,
			GoVer:    defaultGoVersion,
			GoPath:   goPath,
			AppVer:   v,
			Fields:   fields,
		})
	}
	return nil
}

func (i *Inspector) InspectStdlib(r Renderer, fields []StructField) error {
	for _, v := range goVersions {
		goPath, err := i.storage.getGo(v.Original())
		if err != nil {
			return err
		}

		i.manifests = append(i.manifests, manifest{
			Renderer: r,
			GoVer:    v.Original(),
			GoPath:   goPath,
			AppVer:   v,
			Fields:   fields,
		})
	}
	return nil
}

func (i *Inspector) Do(ctx context.Context) (*schema.TrackedOffsets, error) {
	g, ctx := errgroup.WithContext(ctx)
	todo := make(chan manifest)

	g.Go(func() error {
		defer close(todo)
		for _, m := range i.manifests {
			select {
			case todo <- m:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	c := make(chan []structFieldOffset)
	for n := 0; n < i.NWorkers; n++ {
		g.Go(func() error {
			for m := range todo {
				out, err := i.do(m)
				if err != nil {
					return err
				}

				select {
				case c <- out:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(c)
	}()

	var results []structFieldOffset
	for r := range c {
		results = append(results, r...)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	i.logResults(results)
	return trackedOffsets(results), nil
}

func (i *Inspector) do(m manifest) ([]structFieldOffset, error) {
	var out []structFieldOffset

	uncached := m.Fields[:0] // Use the same backing array.
	for _, f := range m.Fields {
		if sfo, ok := i.cached(m.AppVer, f); ok {
			out = append(out, sfo)
		} else {
			uncached = append(uncached, f)
		}
	}

	if len(uncached) == 0 {
		return out, nil
	}

	d, err := os.MkdirTemp("", "inspect-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(d)

	data := struct{ Version string }{Version: "v" + m.AppVer.String()}
	if err = m.Renderer.Render(d, data); err != nil {
		return nil, err
	}

	app, err := i.builder.Build(d, m.GoPath, m.AppVer)
	if errors.Is(err, errBuild) {
		for _, f := range uncached {
			i.log.Error(err, "skipping", "field", f, "Go", m.GoVer, "version", m.AppVer)
			out = append(out, structFieldOffset{
				StructField: f,
				Version:     m.AppVer,
				Offset:      -1,
			})
		}
		return out, nil
	} else if err != nil {
		return nil, err
	}
	defer app.Close()

	for _, f := range uncached {
		out = append(out, app.Analyze(f))
	}

	return out, nil
}

func (i *Inspector) cached(ver *version.Version, sf StructField) (structFieldOffset, bool) {
	sfo := structFieldOffset{StructField: sf, Version: ver}

	var ok bool
	sfo.Offset, ok = i.cache.Get(ver.String(), sf.Package, sf.Struct, sf.Field)
	return sfo, ok
}

func (i *Inspector) logResults(results []structFieldOffset) {
	for _, r := range results {
		if r.Offset < 0 {
			i.log.Info("offsets not found", "name", r.structName(), "version", r.Version)
		} else {
			i.log.Info("offsets found", "name", r.structName(), "version", r.Version)
		}
	}
}
