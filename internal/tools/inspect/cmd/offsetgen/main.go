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

	"go.opentelemetry.io/auto/internal/pkg/structfield"
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

	goOtelVers, err := PkgVersions("go.opentelemetry.io/otel")
	if err != nil {
		return nil, fmt.Errorf("failed to get \"go.opentelemetry.io/otel\" versions: %w", err)
	}

	kafkaGoVers, err := PkgVersions("github.com/segmentio/kafka-go")
	if err != nil {
		return nil, fmt.Errorf("failed to get \"github.com/segmentio/kafka-go\" versions: %w", err)
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
			StructFields: []structfield.ID{
				structfield.NewID("std", "runtime", "g", "goid"),
				structfield.NewID("std", "runtime", "hmap", "buckets"),
			},
		},
		{
			Application: inspect.Application{
				Renderer:  ren("templates/net/http/*.tmpl"),
				GoVerions: goVers,
			},
			StructFields: []structfield.ID{
				structfield.NewID("std", "net/http", "Request", "Method"),
				structfield.NewID("std", "net/http", "Request", "URL"),
				structfield.NewID("std", "net/http", "Request", "RemoteAddr"),
				structfield.NewID("std", "net/http", "Request", "Header"),
				structfield.NewID("std", "net/http", "Request", "ctx"),
				structfield.NewID("std", "net/http", "Response", "StatusCode"),
				structfield.NewID("std", "net/http", "response", "req"),
				structfield.NewID("std", "net/http", "response", "status"),
				structfield.NewID("std", "net/http", "Request", "Proto"),
				structfield.NewID("std", "net/http", "Request", "RequestURI"),
				structfield.NewID("std", "net/http", "Request", "Host"),
				structfield.NewID("std", "net/http", "Request", "pat"),
				structfield.NewID("std", "net/http", "pattern", "str"),
				structfield.NewID("std", "net/url", "URL", "Path"),
				structfield.NewID("std", "net/url", "URL", "Scheme"),
				structfield.NewID("std", "net/url", "URL", "Opaque"),
				structfield.NewID("std", "net/url", "URL", "User"),
				structfield.NewID("std", "net/url", "URL", "Host"),
				structfield.NewID("std", "net/url", "URL", "RawPath"),
				structfield.NewID("std", "net/url", "URL", "OmitHost"),
				structfield.NewID("std", "net/url", "URL", "ForceQuery"),
				structfield.NewID("std", "net/url", "URL", "RawQuery"),
				structfield.NewID("std", "net/url", "URL", "Fragment"),
				structfield.NewID("std", "net/url", "URL", "RawFragment"),
				structfield.NewID("std", "bufio", "Writer", "buf"),
				structfield.NewID("std", "bufio", "Writer", "n"),
			},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/google.golang.org/grpc/*.tmpl"),
				Versions: grpcVers,
			},
			StructFields: []structfield.ID{
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "method"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "id"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "Stream", "ctx"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc", "ClientConn", "target"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "http2Client", "nextID"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "streamID"),
				structfield.NewID("google.golang.org/grpc", "google.golang.org/grpc/internal/transport", "headerFrame", "hf"),
			},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/golang.org/x/net/*.tmpl"),
				Versions: xNetVers,
			},
			StructFields: []structfield.ID{
				structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "MetaHeadersFrame", "Fields"),
				structfield.NewID("golang.org/x/net", "golang.org/x/net/http2", "FrameHeader", "StreamID"),
			},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/go.opentelemetry.io/otel/traceglobal/*.tmpl"),
				Versions: goOtelVers,
			},
			StructFields: []structfield.ID{
				structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracer", "delegate"),
			},
		},
		{
			Application: inspect.Application{
				Renderer: ren("templates/github.com/segmentio/kafka-go/*.tmpl"),
				Versions: kafkaGoVers,
			},
			StructFields: []structfield.ID{
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Topic"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Partition"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Offset"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Key"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Headers"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Time"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Writer", "Topic"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Reader", "config"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "ReaderConfig", "GroupID"),
				structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Conn", "clientID"),
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
