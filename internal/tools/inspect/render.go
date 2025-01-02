// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/golang.org/x/net/*.tmpl
//go:embed templates/google.golang.org/grpc/*.tmpl
//go:embed templates/net/http/*.tmpl
//go:embed templates/runtime/*.tmpl
//go:embed templates/go.opentelemetry.io/otel/traceglobal/*.tmpl
//go:embed templates/github.com/segmentio/kafka-go/*.tmpl
//go:embed templates/github.com/redis/rueidis/*.tmpl
var DefaultFS embed.FS

// Renderer renders templates from an fs.FS.
type Renderer struct {
	log *slog.Logger

	fs  fs.FS
	src string
}

// NewRenderer returns a new *Renderer used to render the template files found
// in f at the provided src.
//
// If f is nil, DefaultFS will be used instead.
func NewRenderer(l *slog.Logger, src string, f fs.FS) Renderer {
	if f == nil {
		f = DefaultFS
	}
	return Renderer{log: l, fs: f, src: src}
}

// Render renders the Renderer's src in dest using data.
//
// All src will be rendered in the same file-tree with the same names (except
// for any ".tmpl" suffixes) as found in the Renderer's fs.FS.
func (r Renderer) Render(dest string, data interface{}) error {
	r.log.Debug("rendering...", "src", r.src, "dest", dest, "data", data)

	tmpls, err := template.ParseFS(r.fs, r.src)
	if err != nil {
		return err
	}
	for _, tmpl := range tmpls.Templates() {
		r.log.Debug("rendering template...", "template", tmpl.Name)
		target := filepath.Join(dest, strings.TrimSuffix(tmpl.Name(), ".tmpl"))
		wr, err := os.Create(target)
		if err != nil {
			return err
		}

		err = tmpl.Execute(wr, data)
		if err != nil {
			return err
		}
		r.log.Debug("rendered template", "template", tmpl.Name())
	}

	r.log.Debug("rendered", "src", r.src, "dest", dest, "data", data)
	return nil
}
