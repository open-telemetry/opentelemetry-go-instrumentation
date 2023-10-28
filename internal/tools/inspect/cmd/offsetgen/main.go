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

// Offsetgen is a utility to generate a static file containing offsets for Go
// struct fields.
package main

import (
	"context"
	"encoding/json"
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
	defaultOutputFile = "offset_results.json"

	minGoVersion = "1.12"
)

var (
	// outputFile is the output file path flag value.
	outputFile string
	// cacheFile is the offset cache file path flag value.
	cacheFile string
	// numCPU is the number of CPUs to use flag value.
	numCPU int
	// verbosity is the log verbosity level flag value.
	verbosity int

	logger logr.Logger
)

func init() {
	flag.StringVar(&outputFile, "output", defaultOutputFile, "output file")
	flag.StringVar(&cacheFile, "cache", "", "offset cache")
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

	grpcVers, err := PkgVersions("google.golang.org/grpc")
	if err != nil {
		return nil, fmt.Errorf("failed to get \"google.golang.org/grpc\" versions: %w", err)
	}

	ren := func(src string) inspect.Renderer {
		return inspect.NewRenderer(logger, src, inspect.DefaultFS)
	}

	return []inspect.Manifest{
		{
			Application: inspect.Application{
				Renderer:  ren("templates/runtime/*.tmpl"),
				GoVerions: goVers,
			},
			StructFields: []inspect.StructField{{
				PkgPath: "runtime",
				Struct:  "g",
				Field:   "goid",
			}, {
				PkgPath: "runtime",
				Struct:  "hmap",
				Field:   "buckets",
			}},
		},
		{
			Application: inspect.Application{
				Renderer:  ren("templates/net/http/*.tmpl"),
				GoVerions: goVers,
			},
			StructFields: []inspect.StructField{{
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "Method",
			}, {
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "URL",
			}, {
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "RemoteAddr",
			}, {
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "Header",
			}, {
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "ctx",
			}, {
				PkgPath: "net/http",
				Struct:  "response",
				Field:   "req",
			}, {
				PkgPath: "net/url",
				Struct:  "URL",
				Field:   "Path",
			}},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/google.golang.org/grpc/*.tmpl"),
				Versions: grpcVers,
			},
			StructFields: []inspect.StructField{{
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "method",
			}, {
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "id",
			}, {
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "ctx",
			}, {
				PkgPath: "google.golang.org/grpc",
				Struct:  "ClientConn",
				Field:   "target",
			}, {
				PkgPath: "golang.org/x/net/http2",
				Struct:  "MetaHeadersFrame",
				Field:   "Fields",
			}, {
				PkgPath: "golang.org/x/net/http2",
				Struct:  "FrameHeader",
				Field:   "StreamID",
			}, {
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "http2Client",
				Field:   "nextID",
			}, {
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "headerFrame",
				Field:   "streamID",
			}, {
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "headerFrame",
				Field:   "hf",
			}},
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

	var cache *inspect.Cache
	if cacheFile != "" {
		cache, err = inspect.NewCache(logger, cacheFile)
		if err != nil {
			logger.Error(err, "failed to load cache", "path", cacheFile)
			// Use an empty cache.
		}
	}

	i, err := inspect.New(logger, cache, m...)
	if err != nil {
		logger.Error(err, "failed to setup inspector")
		return err
	}
	i.NWorkers = numCPU

	// Trap Ctrl+C and call cancel on the context.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	to, err := i.Do(ctx)
	if err != nil {
		logger.Error(err, "failed get offsets")
		return err
	}

	if to == nil {
		logger.Info("no offsets found")
		return nil
	}

	logger.Info("writing offsets", "dest", outputFile)
	f, err := os.Create(outputFile)
	if err != nil {
		logger.Error(err, "failed to open output file", "dest", outputFile)
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(to); err != nil {
		logger.Error(err, "failed to write offsets", "dest", outputFile)
		return err
	}
	return nil
}
