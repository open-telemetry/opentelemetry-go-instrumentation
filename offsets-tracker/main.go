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
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/offsets-tracker/binary"
	"go.opentelemetry.io/auto/offsets-tracker/target"
	"go.opentelemetry.io/auto/offsets-tracker/writer"
)

const (
	defaultOutputFile = "/tmp/offset_results.json"
)

func main() {
	outputFilename := defaultOutputFile
	if len(os.Getenv("OFFSETS_OUTPUT_FILE")) > 0 {
		outputFilename = os.Getenv("OFFSETS_OUTPUT_FILE")
	}
	outputFile := flag.String("output", outputFilename, "output file")
	flag.Parse()

	minimunGoVersion, err := version.NewConstraint(">= 1.12")
	if err != nil {
		log.Fatalf("error in parsing version constraint: %v\n", err)
	}

	stdLibOffsets, err := target.New("go", *outputFile).
		FindVersionsBy(target.GoDevFileVersionsStrategy).
		DownloadBinaryBy(target.DownloadPreCompiledBinaryFetchStrategy).
		VersionConstraint(&minimunGoVersion).
		FindOffsets([]*binary.DataMember{
			{
				StructName: "runtime.g",
				Field:      "goid",
			},
			{
				StructName: "net/http.Request",
				Field:      "Method",
			},
			{
				StructName: "net/http.Request",
				Field:      "URL",
			},
			{
				StructName: "net/http.Request",
				Field:      "RemoteAddr",
			},
			{
				StructName: "net/http.Request",
				Field:      "ctx",
			},
			{
				StructName: "net/url.URL",
				Field:      "Path",
			},
		})

	if err != nil {
		log.Fatalf("error while fetching offsets: %v\n", err)
	}

	grpcOffsets, err := target.New("google.golang.org/grpc", *outputFile).
		FindOffsets([]*binary.DataMember{
			{
				StructName: "google.golang.org/grpc/internal/transport.Stream",
				Field:      "method",
			},
			{
				StructName: "google.golang.org/grpc/internal/transport.Stream",
				Field:      "id",
			},
			{
				StructName: "google.golang.org/grpc/internal/transport.Stream",
				Field:      "ctx",
			},
			{
				StructName: "google.golang.org/grpc.ClientConn",
				Field:      "target",
			},
			{
				StructName: "golang.org/x/net/http2.MetaHeadersFrame",
				Field:      "Fields",
			},
			{
				StructName: "golang.org/x/net/http2.FrameHeader",
				Field:      "StreamID",
			},
		})

	if err != nil {
		log.Fatalf("error while fetching offsets: %v\n", err)
	}

	fmt.Println("Done collecting offsets, writing results to file ...")
	err = writer.WriteResults(*outputFile, stdLibOffsets, grpcOffsets)
	if err != nil {
		log.Fatalf("error while writing results to file: %v\n", err)
	}

	fmt.Println("Done!")
}
