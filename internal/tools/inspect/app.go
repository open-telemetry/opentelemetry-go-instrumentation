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
)

type app struct {
	Manifest manifest

	log    logr.Logger
	tmpDir string
	exec   string
	data   *dwarf.Data
}

func newApp(ctx context.Context, l logr.Logger, m manifest) (*app, error) {
	a := &app{log: l.WithName("app"), Manifest: m}

	var err error
	a.tmpDir, err = os.MkdirTemp("", "inspect-*")
	if err != nil {
		return nil, err
	}

	data := struct{ Version string }{
		Version: "v" + a.Manifest.AppVer.String(),
	}
	if err = m.Renderer.Render(a.tmpDir, data); err != nil {
		return nil, err
	}

	a.exec, err = m.Builder.Build(ctx, a.tmpDir, a.Manifest.AppVer)
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

func (a *app) Analyze(sf StructField) structFieldOffset {
	a.log.V(1).Info("analyzing binary...", "package", sf.Package, "binary", a.exec)
	return sf.offset(a.Manifest.AppVer, a.data)
}

func (a *app) Close() error {
	return os.RemoveAll(a.tmpDir)
}
