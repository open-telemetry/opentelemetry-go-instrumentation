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
	"os"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
)

type app struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version
	Fields   []StructField

	log    logr.Logger
	tmpDir string
	exec   string
	data   *dwarf.Data
}

func newApp(ctx context.Context, l logr.Logger, j job) (*app, error) {
	a := &app{
		Renderer: j.Renderer,
		Builder:  j.Builder,
		AppVer:   j.AppVer,
		Fields:   j.Fields,
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

func (a *app) GetOffset(sf StructField) (uint64, bool) {
	a.log.V(1).Info("analyzing binary...", "package", sf.PkgPath, "binary", a.exec)
	return sf.offset(a.data)
}

func (a *app) Close() error {
	return os.RemoveAll(a.tmpDir)
}
