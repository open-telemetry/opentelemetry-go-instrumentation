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

	"go.opentelemetry.io/auto/internal/pkg/inject"
)

const defaultNWorkers = 20

type manifest struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version
	Fields   []StructField
}

// Inspector inspects structure of Go packages.
type Inspector struct {
	NWorkers int

	log    logr.Logger
	cache  *cache
	client *client.Client

	manifests []manifest
}

// New returns an Inspector that will use offsetFile as a cache for offsets.
func New(l logr.Logger, offsetFile string) (*Inspector, error) {
	logger := l.WithName("inspector")

	c, err := newCache(l, offsetFile)
	if err != nil {
		logger.Error(err, "using empty cache")
	}

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	return &Inspector{
		NWorkers: defaultNWorkers,
		log:      logger,
		cache:    c,
		client:   cli,
	}, nil
}

// Inspect3rdParty adds fields for a 3rd-party package to be a analyzed for the
// passed vers of that package using the Renderer r to generate a token
// program. The token program needs to contain the fields when compiled for the
// analysis to work.
func (i *Inspector) Inspect3rdParty(r Renderer, vers []*version.Version, fields []StructField) {
	for _, v := range vers {
		i.manifests = append(i.manifests, manifest{
			Renderer: r,
			Builder:  newBuilder(i.log, i.client, nil),
			AppVer:   v,
			Fields:   fields,
		})
	}
}

// InspectStdlib adds fields for a stdlib package to be a analyzed for the
// passed vers using the Renderer r to generate a token program. The token
// program needs to contain the fields when compiled for the analysis to work.
func (i *Inspector) InspectStdlib(r Renderer, vers []*version.Version, fields []StructField) {
	for _, v := range vers {
		i.manifests = append(i.manifests, manifest{
			Renderer: r,
			Builder:  newBuilder(i.log, i.client, v),
			AppVer:   v,
			Fields:   fields,
		})
	}
}

// Do performs the inspections and returns all found offsets.
func (i *Inspector) Do(ctx context.Context) (*inject.TrackedOffsets, error) {
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

	var results []structFieldOffset
	for r := range c {
		i.logResults(r)
		results = append(results, r...)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return trackedOffsets(results), nil
}

func (i *Inspector) do(ctx context.Context, m manifest) ([]structFieldOffset, error) {
	var (
		missing bool
		out     []structFieldOffset
	)
	for _, f := range m.Fields {
		sfo := i.cache.Get(m.AppVer, f)
		out = append(out, sfo)
		if sfo.Offset < 0 {
			missing = true
		}
	}

	if !missing {
		return out, nil
	}

	app, err := newApp(ctx, i.log, m)
	buildErr := &errBuild{}
	if errors.As(err, &buildErr) {
		i.log.V(1).Info(
			"failed to build app, skipping",
			"version", m.AppVer,
			"src", m.Renderer.src,
			"Go", m.Builder.GoImage,
			"rc", buildErr.ReturnCode,
			"stdout", buildErr.Stdout,
			"stderr", buildErr.Stderr,
		)
		return out, nil
	} else if err != nil {
		return nil, err
	}
	defer app.Close()

	for i, f := range out {
		if f.Offset < 0 {
			out[i] = app.Analyze(f.StructField)
		}
	}

	return out, nil
}

func (i *Inspector) logResults(results []structFieldOffset) {
	for _, r := range results {
		msg := "offset "
		kv := []interface{}{
			"version", r.Version,
			"package", r.Package,
			"struct", r.Struct,
			"field", r.Field,
		}
		if r.Offset < 0 {
			msg += "not found"
		} else {
			msg += "found"
			kv = append(kv, "offset", r.Offset)
		}
		i.log.Info(msg, kv...)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
