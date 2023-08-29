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
	"flag"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"go.opentelemetry.io/auto/offsets-tracker/binary"
	"go.opentelemetry.io/auto/offsets-tracker/cache"
	"go.opentelemetry.io/auto/offsets-tracker/inspect"
	"go.opentelemetry.io/auto/offsets-tracker/versions"
	"go.opentelemetry.io/auto/offsets-tracker/writer"
)

const (
	defaultOutputFile = "/tmp/offset_results.json"

	minGoVersion     = "1.12"
	defaultGoVersion = "1.21"
)

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
	l := stdr.New(log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile))
	c := cache.Load(outputFile)

	i, err := inspect.New(l, c, storage)
	if err != nil {
		l.Error(err, "failed to setup inspector")
		os.Exit(1)
	}

	var offsets []*inspect.Offsets

	o, err := i.StdlibOffsets(
		"runtime",
		"templates/runtime/*.tmpl",
		[]*binary.DataMember{{
			StructName: "runtime.g",
			Field:      "goid",
		}},
	)
	if err != nil {
		l.Error(err, "failed to fetch runtime offsets")
	} else {
		offsets = append(offsets, o...)
	}

	o, err = i.StdlibOffsets(
		"net/http",
		"templates/net/http/*.tmpl",
		[]*binary.DataMember{{
			StructName: "net/http.Request",
			Field:      "Method",
		}, {
			StructName: "net/http.Request",
			Field:      "URL",
		}, {
			StructName: "net/http.Request",
			Field:      "RemoteAddr",
		}, {
			StructName: "net/http.Request",
			Field:      "Header",
		}, {
			StructName: "net/http.Request",
			Field:      "ctx",
		}},
	)
	if err != nil {
		l.Error(err, "failed to fetch net/http offsets")
	} else {
		offsets = append(offsets, o...)
	}

	o, err = i.StdlibOffsets(
		"net/url",
		"templates/net/http/*.tmpl",
		[]*binary.DataMember{{
			StructName: "net/url.URL",
			Field:      "Path",
		}},
	)
	if err != nil {
		l.Error(err, "failed to fetch net/url offsets")
	} else {
		offsets = append(offsets, o...)
	}

	o, err = i.Offsets(
		"google.golang.org/grpc",
		"templates/google.golang.org/grpc/*.tmpl",
		versions.List("google.golang.org/grpc"),
		[]*binary.DataMember{{
			StructName: "google.golang.org/grpc/internal/transport.Stream",
			Field:      "method",
		}, {
			StructName: "google.golang.org/grpc/internal/transport.Stream",
			Field:      "id",
		}, {
			StructName: "google.golang.org/grpc/internal/transport.Stream",
			Field:      "ctx",
		}, {
			StructName: "google.golang.org/grpc.ClientConn",
			Field:      "target",
		}, {
			StructName: "golang.org/x/net/http2.MetaHeadersFrame",
			Field:      "Fields",
		}, {
			StructName: "golang.org/x/net/http2.FrameHeader",
			Field:      "StreamID",
		}, {
			StructName: "google.golang.org/grpc/internal/transport.http2Client",
			Field:      "nextID",
		}, {
			StructName: "google.golang.org/grpc/internal/transport.headerFrame",
			Field:      "streamID",
		}, {
			StructName: "google.golang.org/grpc/internal/transport.headerFrame",
			Field:      "hf",
		}},
	)
	if err != nil {
		l.Error(err, "failed to fetch net/url offsets")
	} else {
		offsets = append(offsets, o...)
	}

	if len(offsets) > 0 {
		l.Info("writing offsets", "dest", outputFile)
		err = writer.WriteResults(outputFile, offsets...)
		if err != nil {
			l.Error(err, "failed to write offsets", "dest", outputFile)
			os.Exit(1)
		}
	}
}
