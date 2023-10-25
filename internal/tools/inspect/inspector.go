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

	"github.com/docker/docker/client"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"golang.org/x/sync/errgroup"

	"go.opentelemetry.io/auto/internal/pkg/offsets"
)

const defaultNWorkers = 20

// Inspector inspects structure of Go packages.
type Inspector struct {
	NWorkers int
	Cache    *Cache

	log    logr.Logger
	client *client.Client

	jobs []job
}

// New returns an Inspector that configured to inspect offsets defined in the
// manifests.
//
// If cache is non-nil, offsets will first be looked up there. Otherwise, the
// offsets will be found by building the applicatiions in the manifests and
// inspecting the produced binaries.
func New(l logr.Logger, cache *Cache, manifests ...Manifest) (*Inspector, error) {
	logger := l.WithName("inspector")

	if cache == nil {
		logger.Info("using empty cache")
		cache = newCache(l)
	}

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	i := &Inspector{
		NWorkers: defaultNWorkers,
		log:      logger,
		Cache:    cache,
		client:   cli,
	}
	for _, m := range manifests {
		err := i.AddManifest(m)
		if err != nil {
			return nil, err
		}
	}
	return i, nil
}

// AddManifest adds the manifest to the Inspector's set of Manifests to
// inspect.
func (i *Inspector) AddManifest(manifest Manifest) error {
	if err := manifest.validate(); err != nil {
		return err
	}

	i.log.V(2).Info("adding manifest", "manifest", manifest)

	goVer := manifest.Application.GoVerions
	if goVer == nil {
		// Passsing nil to newBuilder will mean the application is built with
		// the latest version of Go.
		b := newBuilder(i.log, i.client, nil)
		for _, ver := range manifest.Application.Versions {
			v := ver
			i.jobs = append(i.jobs, job{
				Renderer: manifest.Application.Renderer,
				Builder:  b,
				AppVer:   v,
				Fields:   manifest.StructFields,
			})
		}
		return nil
	}

	if manifest.Application.Versions == nil {
		for _, gVer := range goVer {
			v := gVer
			i.jobs = append(i.jobs, job{
				Renderer: manifest.Application.Renderer,
				Builder:  newBuilder(i.log, i.client, v),
				AppVer:   v,
				Fields:   manifest.StructFields,
			})
		}
		return nil
	}

	for _, gV := range goVer {
		b := newBuilder(i.log, i.client, gV)
		for _, ver := range manifest.Application.Versions {
			v := ver
			i.jobs = append(i.jobs, job{
				Renderer: manifest.Application.Renderer,
				Builder:  b,
				AppVer:   v,
				Fields:   manifest.StructFields,
			})
		}
	}
	return nil
}

type job struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version
	Fields   []StructField
}

// Do performs the inspections and returns all found offsets.
func (i *Inspector) Do(ctx context.Context) (*offsets.Index, error) {
	g, ctx := errgroup.WithContext(ctx)
	todo := make(chan job)

	g.Go(func() error {
		defer close(todo)
		for _, j := range i.jobs {
			select {
			case todo <- j:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	c := make(chan []result)
	for n := 0; n < max(1, i.NWorkers-1); n++ {
		g.Go(func() error {
			for m := range todo {
				out, err := i.do(ctx, m)
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
		_ = g.Wait()
		close(c)
	}()

	index := offsets.NewIndex()
	for results := range c {
		for _, r := range results {
			i.logResult(r)
			if !r.Found {
				continue
			}

			index.PutOffset(r.StructField.id(), r.Version, r.Offset)
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return index, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type result struct {
	StructField StructField
	Version     *version.Version
	Offset      uint64
	Found       bool
}

func (i *Inspector) do(ctx context.Context, j job) (out []result, err error) {
	var uncachedIndices []int
	for _, f := range j.Fields {
		o, ok := i.Cache.GetOffset(j.AppVer, f)
		out = append(out, result{
			StructField: f,
			Version:     j.AppVer,
			Offset:      o,
			Found:       ok,
		})
		if !ok {
			uncachedIndices = append(uncachedIndices, len(out)-1)
		}
	}

	if len(uncachedIndices) == 0 {
		return out, nil
	}

	app, err := newApp(ctx, i.log, j)
	buildErr := &errBuild{}
	if errors.As(err, &buildErr) {
		i.log.V(1).Info(
			"failed to build app, skipping",
			"version", j.AppVer,
			"src", j.Renderer.src,
			"Go", j.Builder.GoImage,
			"rc", buildErr.ReturnCode,
			"stdout", buildErr.Stdout,
			"stderr", buildErr.Stderr,
		)
		return out, nil
	} else if err != nil {
		return nil, err
	}
	defer app.Close()

	for _, i := range uncachedIndices {
		out[i].Offset, out[i].Found = app.GetOffset(out[i].StructField)
	}

	return out, nil
}

func (i *Inspector) logResult(r result) {
	msg := "offset "
	kv := []interface{}{
		"version", r.Version,
		"package", r.StructField.PkgPath,
		"struct", r.StructField.Struct,
		"field", r.StructField.Field,
	}
	if !r.Found {
		msg += "not found"
	} else {
		msg += "found"
		kv = append(kv, "offset", r.Offset)
	}
	i.log.Info(msg, kv...)
}
