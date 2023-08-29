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
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/auto/offsets-tracker/binary"
	"go.opentelemetry.io/auto/offsets-tracker/cache"
	"go.opentelemetry.io/auto/offsets-tracker/versions"
)

const (
	defaultStoragePath = "./.offset-tracker"

	// TODO: minGoVersion     = "1.12"
	minGoVersion     = "1.20"
	defaultGoVersion = "1.21.0"

	shell = "bash"
)

var (
	// goVersions are the versions of Go supported.
	goVersions []string

	//go:embed templates/google.golang.org/grpc/*.tmpl
	//go:embed templates/net/http/*.tmpl
	//go:embed templates/runtime/*.tmpl
	rootFS embed.FS
)

func init() {
	var err error
	goVersions, err = versions.Go(">= " + minGoVersion)
	if err != nil {
		fmt.Printf("failed to get Go versions: %v", err)
		os.Exit(1)
	}
}

type Inspector struct {
	log     logr.Logger
	cache   *cache.Cache
	storage *storage
}

func New(l logr.Logger, c *cache.Cache, storage string) (*Inspector, error) {
	if c == nil {
		c = cache.New()
	}

	if storage == "" {
		storage = defaultStoragePath
	}

	s, err := newStorage(l, storage)
	if err != nil {
		return nil, err
	}

	return &Inspector{log: l, cache: c, storage: s}, nil
}

func (i *Inspector) Offsets(pkg, tmplSrc string, versions func() ([]string, error), dm []*binary.DataMember) ([]*Offsets, error) {
	results := []*Offsets{{ModuleName: pkg}}

	vers, err := versions()
	if err != nil {
		return nil, err
	}

	goPath, err := i.storage.getGo(defaultGoVersion)
	if err != nil {
		return nil, err
	}

	for _, v := range vers {
		if res, ok := i.lookup(pkg, v, dm); ok {
			results = i.append(pkg, v, results, res)
			continue
		}

		d, err := os.MkdirTemp("", "offset-tracker-*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(d)

		err = render(tmplSrc, d, struct{ Version string }{Version: v})
		if err != nil {
			return nil, err
		}

		exec, err := i.build(goPath, d)
		if errors.Is(err, errBuild) {
			i.log.Info("could not build app", "package", pkg, "version", v)
			results = i.append(pkg, v, results, nil)
			continue
		} else if err != nil {
			return nil, err
		}

		res, err := i.analyze(pkg, v, exec, dm)
		if err != nil {
			return nil, err
		}
		results = i.append(pkg, v, results, res)
	}
	if len(results[len(results)-1].ResultsByVersion) == 0 {
		results = results[:len(results)-1]
	}
	return results, nil
}

func (i *Inspector) StdlibOffsets(pkg, tmplSrc string, dm []*binary.DataMember) ([]*Offsets, error) {
	results := []*Offsets{{ModuleName: pkg}}
	for _, v := range goVersions {
		if res, ok := i.lookup(pkg, v, dm); ok {
			results = i.append(pkg, v, results, res)
			continue
		}

		goPath, err := i.storage.getGo(v)
		if err != nil {
			return nil, err
		}

		d, err := os.MkdirTemp("", "offset-tracker-*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(d)

		if err = render(tmplSrc, d, nil); err != nil {
			return nil, err
		}

		exec, err := i.build(goPath, d)
		if errors.Is(err, errBuild) {
			i.log.Info("could not build app", "package", pkg, "version", v)
			results = i.append(pkg, v, results, nil)
			continue
		} else if err != nil {
			return nil, err
		}

		res, err := i.analyze(pkg, v, exec, dm)
		if err != nil {
			return nil, err
		}
		results = i.append(pkg, v, results, res)
	}
	if len(results[len(results)-1].ResultsByVersion) == 0 {
		results = results[:len(results)-1]
	}
	return results, nil
}

func (i *Inspector) lookup(pkg, ver string, dm []*binary.DataMember) (*VersionedResult, bool) {
	cached, ok := i.cache.IsAllInCache(ver, dm)
	if !ok {
		i.log.Info("cache miss", "package", pkg, "version", ver)
		return nil, false
	}

	i.log.Info("cache hit", "package", pkg, "version", ver)
	return &VersionedResult{
		Version:    ver,
		OffsetData: &binary.Result{DataMembers: cached},
	}, true
}

func (i *Inspector) append(pkg, ver string, off []*Offsets, r *VersionedResult) []*Offsets {
	if r == nil {
		i.log.Info("no offsets found", "package", pkg, "version", ver)
		if len(off) > 0 && len(off[0].ResultsByVersion) == 0 {
			// Nothing has been recorded yet.
			return off
		}
		return append(off, &Offsets{ModuleName: pkg})
	}

	i.log.Info("offsets found", "package", pkg, "version", ver)
	if len(off) == 0 {
		return []*Offsets{{ModuleName: pkg}}
	}
	off[len(off)-1].ResultsByVersion = append(off[len(off)-1].ResultsByVersion, r)
	return off
}

func (i *Inspector) analyze(pkg, ver, exec string, dm []*binary.DataMember) (*VersionedResult, error) {
	i.log.Info("analyzing app binary", "package", pkg, "binary", exec)
	f, err := os.Open(exec)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res, err := binary.FindOffsets(f, dm)
	if err == binary.ErrOffsetsNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &VersionedResult{Version: ver, OffsetData: res}, nil
}

var errBuild = errors.New("failed to build")

func (i *Inspector) build(goBin, dir string) (string, error) {
	goModTidy := goBin + " mod tidy -compat=1.17"
	stdout, stderr, err := run(goModTidy, dir)
	if err != nil {
		i.log.Error(
			err, "failed to tidy application",
			"cmd", goModTidy,
			"STDOUT", stdout,
			"STDERR", stderr,
		)
		return "", err
	}

	build := goBin + " build -o app"
	stdout, stderr, err = run(build, dir)
	if err != nil {
		i.log.V(5).Info(
			"failed to build application",
			"cmd", build,
			"STDOUT", stdout,
			"STDERR", stderr,
			"err", err,
		)
		return "", errBuild
	}

	return filepath.Join(dir, "app"), nil
}

// render renders all templates to the dest directory using the data.
func render(src, dest string, data interface{}) error {
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

// run runs command in dir.
func run(command string, dir string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(shell, "-c", command)
	if dir != "" {
		cmd.Dir = dir
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
