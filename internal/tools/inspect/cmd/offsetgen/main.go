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

	xNetVers, err := PkgVersions("golang.org/x/net")
	if err != nil {
		return nil, fmt.Errorf("failed to get \"golang.org/x/net\" versions: %w", err)
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
				ModPath: "std",
				PkgPath: "runtime",
				Struct:  "g",
				Field:   "goid",
			}, {
				ModPath: "std",
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
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "Method",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "URL",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "RemoteAddr",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "Header",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "Request",
				Field:   "ctx",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "response",
				Field:   "req",
			}, {
				ModPath: "std",
				PkgPath: "net/http",
				Struct:  "response",
				Field:   "status",
			}, {
				ModPath: "std",
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
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "method",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "id",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "Stream",
				Field:   "ctx",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc",
				Struct:  "ClientConn",
				Field:   "target",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "http2Client",
				Field:   "nextID",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "headerFrame",
				Field:   "streamID",
			}, {
				ModPath: "google.golang.org/grpc",
				PkgPath: "google.golang.org/grpc/internal/transport",
				Struct:  "headerFrame",
				Field:   "hf",
			}},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/golang.org/x/net/*.tmpl"),
				Versions: xNetVers,
			},
			StructFields: []inspect.StructField{{
				ModPath: "golang.org/x/net",
				PkgPath: "golang.org/x/net/http2",
				Struct:  "MetaHeadersFrame",
				Field:   "Fields",
			}, {
				ModPath: "golang.org/x/net",
				PkgPath: "golang.org/x/net/http2",
				Struct:  "FrameHeader",
				Field:   "StreamID",
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
