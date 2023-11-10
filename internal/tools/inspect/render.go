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
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
)

//go:embed templates/golang.org/x/net/*.tmpl
//go:embed templates/google.golang.org/grpc/*.tmpl
//go:embed templates/net/http/*.tmpl
//go:embed templates/runtime/*.tmpl
//go:embed templates/go.opentelemetry.io/otel/sdk/*.tmpl
var DefaultFS embed.FS

// Renderer renders templates from an fs.FS.
type Renderer struct {
	log logr.Logger

	fs  fs.FS
	src string
}

// NewRenderer returns a new *Renderer used to render the template files found
// in f at the provided src.
//
// If f is nil, DefaultFS will be used instead.
func NewRenderer(l logr.Logger, src string, f fs.FS) Renderer {
	if f == nil {
		f = DefaultFS
	}
	return Renderer{log: l.WithName("renderer"), fs: f, src: src}
}

// Render renders the Renderer's src in dest using data.
//
// All src will be rendered in the same file-tree with the same names (except
// for any ".tmpl" suffixes) as found in the Renderer's fs.FS.
func (r Renderer) Render(dest string, data interface{}) error {
	r.log.V(2).Info("rendering...", "src", r.src, "dest", dest, "data", data)

	tmpls, err := template.ParseFS(r.fs, r.src)
	if err != nil {
		return err
	}
	for _, tmpl := range tmpls.Templates() {
		r.log.V(3).Info("rendering template...", "template", tmpl.Name)
		target := filepath.Join(dest, strings.TrimSuffix(tmpl.Name(), ".tmpl"))
		wr, err := os.Create(target)
		if err != nil {
			return err
		}

		err = tmpl.Execute(wr, data)
		if err != nil {
			return err
		}
		r.log.V(2).Info("rendered template", "template", tmpl.Name())
	}

	r.log.V(1).Info("rendered", "src", r.src, "dest", dest, "data", data)
	return nil
}
