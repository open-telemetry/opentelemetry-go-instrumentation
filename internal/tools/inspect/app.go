// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"context"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"log/slog"
	"os"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// app holds a built Go application.
type app struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version
	Fields   []structfield.ID

	log    *slog.Logger
	tmpDir string
	exec   string
	data   *dwarf.Data
}

// newApp builds and returns a new app.
//
// The new app is built in a temp directory. It is up to the caller to ensure
// the returned app's Close method is called when it is no longer needed so
// all temp directory resources are cleaned up.
func newApp(ctx context.Context, l *slog.Logger, j job) (*app, error) {
	a := &app{
		Renderer: j.Renderer,
		Builder:  j.Builder,
		AppVer:   j.AppVer,
		Fields:   j.Fields,
		log:      l,
	}

	var err error
	a.tmpDir, err = os.MkdirTemp("", "inspect-*")
	if err != nil {
		return nil, err
	}

	data := struct{ Version string }{
		Version: "v" + a.AppVer.String(),
	}
	if err = j.Renderer.Render(a.tmpDir, data); err != nil {
		return nil, err
	}

	if len(j.Fields) == 0 {
		return nil, errors.New("no fields to analyze")
	}
	modName := j.Fields[0].ModPath

	a.exec, err = j.Builder.Build(ctx, a.tmpDir, a.AppVer, modName)
	if err != nil {
		return nil, err
	}

	elfF, err := elf.Open(a.exec)
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	a.data, err = elfF.DWARF()
	if err != nil {
		return nil, err
	}

	a.log.Debug("built app", "binary", a.exec)
	return a, nil
}

// GetOffset returnst the struct field offset for sf. It uses the DWARF data
// of the app's built binary to find this value.
func (a *app) GetOffset(id structfield.ID) (uint64, bool) {
	a.log.Debug("analyzing binary...", "id", id, "binary", a.exec)

	d := process.DWARF{Reader: a.data.Reader()}
	v, err := d.GoStructField(id)
	if err != nil || v < 0 {
		a.log.Error(
			"failed to get offset",
			"error", err,
			"id", id,
			"got", v,
		)
		return 0, false
	}

	return uint64(v), true
}

// Close closes the app, releasing all held resources.
func (a *app) Close() error {
	return os.RemoveAll(a.tmpDir)
}
