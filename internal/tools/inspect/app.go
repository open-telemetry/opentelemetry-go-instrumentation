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
	"context"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"os"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
)

// app holds a built Go application.
type app struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version

	log    logr.Logger
	tmpDir string
	exec   string
	data   *dwarf.Data
}

// newApp builds and returns a new app.
//
// The new app is built in a temp directory. It is up to the caller to ensure
// the returned app's Close method is called when it is no longer needed so
// all temp directory resources are cleaned up.
func newApp(ctx context.Context, l logr.Logger, j job) (*app, error) {
	a := &app{
		Renderer: j.Renderer,
		Builder:  j.Builder,
		AppVer:   j.AppVer,
		log:      l.WithName("app"),
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

	a.exec, err = j.Builder.Build(ctx, a.tmpDir, a.AppVer)
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

	a.log.V(1).Info("built app", "binary", a.exec)
	return a, nil
}

// GetOffset returnst the struct field offset for sf. It uses the DWARF data
// of the app's built binary to find this value.
func (a *app) GetOffset(sf StructField) (uint64, bool) {
	a.log.V(1).Info("analyzing binary...", "package", sf.PkgPath, "binary", a.exec)
	return sf.offset(a.data)
}

func (a *app) HasSubprogram(name string) (bool, error) {
	_, err := findEntry(a.data.Reader(), dwarf.TagSubprogram, name)
	switch {
	case errors.Is(err, errNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// Close closes the app, releasing all held resources.
func (a *app) Close() error {
	return os.RemoveAll(a.tmpDir)
}
