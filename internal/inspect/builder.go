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
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
)

const shell = "bash"

type builder struct {
	log logr.Logger
}

func newBuilder(l logr.Logger) *builder {
	return &builder{log: l}
}

var errBuild = errors.New("failed to build")

func (b *builder) Build(dir, goBin string, ver *version.Version) (*app, error) {
	// TODO: replace with docker build
	goModTidy := goBin + " mod tidy -compat=1.17"
	stdout, stderr, err := b.run(goModTidy, dir)
	if err != nil {
		b.log.Error(
			err, "failed to tidy application",
			"cmd", goModTidy,
			"STDOUT", stdout,
			"STDERR", stderr,
		)
		return nil, err
	}

	app := fmt.Sprintf("app%s", ver.Original())
	build := goBin + " build -o " + app
	stdout, stderr, err = b.run(build, dir)
	if err != nil {
		b.log.V(5).Info(
			"failed to build application",
			"cmd", build,
			"STDOUT", stdout,
			"STDERR", stderr,
			"err", err,
		)
		return nil, errBuild
	}

	return newApp(b.log, ver, filepath.Join(dir, app))
}

// run runs cmd in dir.
func (b *builder) run(cmd string, dir string) (string, string, error) {
	b.log.Info("running command", "cmd", cmd, "dir", dir, "shell", shell)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	c := exec.Command(shell, "-c", cmd)
	if dir != "" {
		c.Dir = dir
	}

	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return stdout.String(), stderr.String(), err
}

type app struct {
	log  logr.Logger
	ver  *version.Version
	exec string
	bin  *elf.File
	data *dwarf.Data
}

func newApp(l logr.Logger, v *version.Version, exec string) (*app, error) {
	elfF, err := elf.Open(exec)
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	dwarfData, err := elfF.DWARF()
	if err != nil {
		return nil, err
	}

	return &app{log: l, ver: v, exec: exec, bin: elfF, data: dwarfData}, nil
}

func (a *app) Analyze(sf StructField) structFieldOffset {
	a.log.Info("analyzing app binary", "package", sf.Package, "binary", a.exec, "version", a.ver)
	return sf.offset(a.ver, a.data)
}

func (a *app) Close() error {
	return a.bin.Close()
}
