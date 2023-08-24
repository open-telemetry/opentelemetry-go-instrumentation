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
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	out = flag.String("out", "../../../../../", "output directory")

	//go:embed templates/*.tmpl
	//go:embed templates/database/sql/*.tmpl
	//go:embed templates/github.com/gin-gonic/gin/*.tmpl
	//go:embed templates/github.com/gorilla/mux/*.tmpl
	//go:embed templates/google.golang.org/grpc/*.tmpl
	//go:embed templates/net/http/*.tmpl
	rootFS embed.FS
)

func main() {
	flag.Parse()

	dest := filepath.Join(*out, "COMPATIBILITY.md")
	fmt.Printf("Generating compatibility documentation at %s ...\n", dest)
	render("templates/COMPATIBILITY.md.tmpl", *out, Packages)

	fmt.Println("Done!")
}

// render renders all templates to the dest directory using the data.
func render(src, dest string, data any) error {
	tmpls, err := template.ParseFS(rootFS, src)
	if err != nil {
		return err
	}
	for _, tmpl := range tmpls.Templates() {
		target := filepath.Join(dest, strings.TrimSuffix(tmpl.Name(), ".tmpl"))
		wr, err := os.Create(target)
		if err != nil {
			return err
		}

		err = tmpl.Execute(wr, data)
		if err != nil {
			return err
		}
	}

	return nil
}
