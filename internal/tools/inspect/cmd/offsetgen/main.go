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
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/go-logr/stdr"
	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/tools/inspect"
)

const (
	defaultOutputFile = "/tmp/offset_results.json"

	// TODO: minGoVersion = "1.12".
	minGoVersion = "1.20"
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

	// goVers are the versions of Go supported.
	goVers []*version.Version
)

func init() {
	outputFilename := defaultOutputFile
	if len(os.Getenv("OFFSETS_OUTPUT_FILE")) > 0 {
		outputFilename = os.Getenv("OFFSETS_OUTPUT_FILE")
	}
	flag.StringVar(&outputFile, "output", outputFilename, "output file")
	flag.StringVar(&cacheFile, "cache", outputFilename, "offset cache")
	flag.IntVar(&numCPU, "workers", runtime.NumCPU(), "max number of Goroutine workers")
	flag.IntVar(&verbosity, "v", 0, "log verbosity")

	flag.Parse()

	var err error
	goVers, err = GoVersions(">= " + minGoVersion)
	if err != nil {
		fmt.Printf("failed to get Go versions: %v", err)
		os.Exit(1)
	}
}

func main() {
	stdr.SetVerbosity(verbosity)
	l := stdr.New(log.New(os.Stdout, "", log.LstdFlags))

	i, err := inspect.New(l, cacheFile)
	if err != nil {
		l.Error(err, "failed to setup inspector")
		os.Exit(1)
	}
	i.NWorkers = numCPU

	ren := func(src string) inspect.Renderer {
		return inspect.NewRenderer(l, src, inspect.DefaultFS)
	}

	i.InspectStdlib(ren("templates/runtime/*.tmpl"), goVers, []inspect.StructField{{
		PkgPath: "runtime",
		Struct:  "g",
		Field:   "goid",
	}})

	i.InspectStdlib(ren("templates/net/http/*.tmpl"), goVers, []inspect.StructField{{
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
		PkgPath: "net/url",
		Struct:  "URL",
		Field:   "Path",
	}})

	v, err := PkgVersions("google.golang.org/grpc")
	if err != nil {
		l.Error(err, "failed to \"google.golang.org/grpc\" versions")
		os.Exit(1)
	}
	i.Inspect3rdParty(ren("templates/google.golang.org/grpc/*.tmpl"), v, []inspect.StructField{{
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
	}})

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
