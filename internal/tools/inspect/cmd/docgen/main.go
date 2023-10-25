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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	"go.opentelemetry.io/auto/internal/tools/inspect"
)

const (
	minGoVersion = "1.12"
)

var (
	// cacheFile is the offset cache file path flag value.
	cacheFile string
	// numCPU is the number of CPUs to use flag value.
	numCPU int
	// verbosity is the log verbosity level flag value.
	verbosity int

	logger logr.Logger
)

func init() {
	flag.StringVar(&cacheFile, "cache", "../../../../pkg/inject/offset_results.json", "offset cache")
	flag.IntVar(&numCPU, "workers", runtime.NumCPU(), "max number of Goroutine workers")
	flag.IntVar(&verbosity, "v", 0, "log verbosity")

	flag.Parse()

	stdr.SetVerbosity(verbosity)
	logger = stdr.New(log.New(os.Stderr, "", log.LstdFlags))
}

func manifests() ([]inspect.Manifest, error) {
	goVers, err := GoVersions(">= " + minGoVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get Go versions: %w", err)
	}

	ren := func(src string) inspect.Renderer {
		return inspect.NewRenderer(logger, src, inspect.DefaultFS)
	}

	return []inspect.Manifest{
		{
			Application: inspect.Application{
				Renderer:  ren("templates/net/http/*.tmpl"),
				GoVerions: goVers,
			},
			Packages: []inspect.Package{
				{
					ImportPath: "net/http",
					Structs: []inspect.Struct{
						{
							Name: "Request",
							Fields: []inspect.Field{
								{Name: "Method"},
								{Name: "URL"},
								{Name: "RemoteAddr"},
								{Name: "Header"},
								{Name: "ctx"},
							},
							Methods: []inspect.Method{
								{Name: "do", Indirect: true},
							},
						},
					},
				},
				{
					ImportPath: "net/url",
					Structs: []inspect.Struct{
						{
							Name: "URL",
							Fields: []inspect.Field{
								{Name: "Path"},
							},
						},
					},
				},
			},
		},
	}, nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	m, err := manifests()
	if err != nil {
		logger.Error(err, "failed to load manifests")
		return err
	}
	logger.V(2).Info("loaded manifests", "manifests", m)

	var cache *inspect.Cache
	if cacheFile != "" {
		cache, err = inspect.NewCache(logger, cacheFile)
		if err != nil {
			logger.Error(err, "failed to load cache", "path", cacheFile)
			// Use an empty cache.
		}
		logger.V(2).Info("loaded cache")
	}

	i, err := inspect.New(logger, cache, m...)
	if err != nil {
		logger.Error(err, "failed to setup inspector")
		return err
	}
	i.NWorkers = numCPU
	logger.V(2).Info("created Inspector", "Inspector", i)

	// Trap Ctrl+C and call cancel on the context.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger.V(1).Info("inspecting supported package versions...")
	to, err := i.Supported(ctx)
	if err != nil {
		logger.Error(err, "failed get supported")
		return err
	}
	logger.V(1).Info("found supported package versions")

	for p, v := range to {
		fmt.Printf("%s: %s\n", p, v)
	}
	return nil
}
