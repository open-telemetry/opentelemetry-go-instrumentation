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
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/go-logr/stdr"
	"go.opentelemetry.io/auto/internal/inspect"
	"go.opentelemetry.io/auto/internal/inspect/cache"
	"go.opentelemetry.io/auto/internal/inspect/versions"
)

const defaultOutputFile = "/tmp/offset_results.json"

var (
	// outputFile is the output file path flag value.
	outputFile string

	// storage is place where Go binaries are stored.
	storage string
)

func init() {
	outputFilename := defaultOutputFile
	if len(os.Getenv("OFFSETS_OUTPUT_FILE")) > 0 {
		outputFilename = os.Getenv("OFFSETS_OUTPUT_FILE")
	}
	flag.StringVar(&outputFile, "output", outputFilename, "output file")

	flag.StringVar(&storage, "storage", "./.offset-tracker", "tooling directory")

	flag.Parse()
}

func main() {
	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile))
	c := cache.Load(l, outputFile)

	i, err := inspect.New(l, c, storage)
	if err != nil {
		l.Error(err, "failed to setup inspector")
		os.Exit(1)
	}
	i.NWorkers = runtime.NumCPU()

	r := inspect.NewRenderer(l, "templates/runtime/*.tmpl", inspect.DefaultFS)
	err = i.InspectStdlib(r, []inspect.StructField{{
		Package: "runtime",
		Struct:  "g",
		Field:   "goid",
	}})
	if err != nil {
		l.Error(err, "failed to add runtime manifest")
		os.Exit(1)
	}

	r = inspect.NewRenderer(l, "templates/net/http/*.tmpl", inspect.DefaultFS)
	err = i.InspectStdlib(r, []inspect.StructField{{
		Package: "net/http",
		Struct:  "Request",
		Field:   "Method",
	}, {
		Package: "net/http",
		Struct:  "Request",
		Field:   "URL",
	}, {
		Package: "net/http",
		Struct:  "Request",
		Field:   "RemoteAddr",
	}, {
		Package: "net/http",
		Struct:  "Request",
		Field:   "Header",
	}, {
		Package: "net/http",
		Struct:  "Request",
		Field:   "ctx",
	}, {
		Package: "net/url",
		Struct:  "URL",
		Field:   "Path",
	}})
	if err != nil {
		l.Error(err, "failed to add net manifest")
		os.Exit(1)
	}

	r = inspect.NewRenderer(l, "templates/google.golang.org/grpc/*.tmpl", inspect.DefaultFS)
	err = i.Inspect3rdParty(r, versions.List("google.golang.org/grpc"), []inspect.StructField{{
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "Stream",
		Field:   "method",
	}, {
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "Stream",
		Field:   "id",
	}, {
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "Stream",
		Field:   "ctx",
	}, {
		Package: "google.golang.org/grpc",
		Struct:  "ClientConn",
		Field:   "target",
	}, {
		Package: "golang.org/x/net/http2",
		Struct:  "MetaHeadersFrame",
		Field:   "Fields",
	}, {
		Package: "golang.org/x/net/http2",
		Struct:  "FrameHeader",
		Field:   "StreamID",
	}, {
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "http2Client",
		Field:   "nextID",
	}, {
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "headerFrame",
		Field:   "streamID",
	}, {
		Package: "google.golang.org/grpc/internal/transport",
		Struct:  "headerFrame",
		Field:   "hf",
	}})
	if err != nil {
		l.Error(err, "failed to add google.golang.org/grpc manifest")
		os.Exit(1)
	}

	// Trap Ctrl+C and call cancel on the context.
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer func() {
		signal.Stop(ch)
		cancel()
	}()
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()

	to, err := i.Do(ctx)
	if err != nil {
		l.Error(err, "failed get offsets")
		os.Exit(1)
	}

	if to != nil {
		l.Info("writing offsets", "dest", outputFile)
		f, err := os.Create(outputFile)
		if err != nil {
			l.Error(err, "failed to open output file", "dest", outputFile)
			os.Exit(1)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(to); err != nil {
			l.Error(err, "failed to write offsets", "dest", outputFile)
			os.Exit(1)
		}
	} else {
		l.Info("no offsets found")
	}
}
