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
	"context"
	"debug/elf"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/auto/internal/inspect/cache"
	"go.opentelemetry.io/auto/internal/inspect/schema"
	"go.opentelemetry.io/auto/internal/inspect/versions"
	"golang.org/x/sync/errgroup"
)

const (
	defaultStoragePath = "./.offset-tracker"

	// TODO: minGoVersion     = "1.12"
	minGoVersion     = "1.20"
	defaultGoVersion = "1.21.0"

	defaultNWorkers = 20

	shell = "bash"
)

var (
	// goVersions are the versions of Go supported.
	goVersions []*version.Version

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

type manifest struct {
	// TODO: replace with appBuilder func() (string, error)
	TmplSrc string
	GoVer   string
	GoPath  string
	AppVer  *version.Version
	Fields  []StructField
}

type Inspector struct {
	NWorkers int

	log     logr.Logger
	cache   *cache.Cache
	storage *storage

	manifests []manifest
}

func New(l logr.Logger, c *cache.Cache, storage string) (*Inspector, error) {
	if c == nil {
		c = cache.New(l)
	}

	if storage == "" {
		storage = defaultStoragePath
	}

	s, err := newStorage(l, storage)
	if err != nil {
		return nil, err
	}

	return &Inspector{
		NWorkers: defaultNWorkers,
		log:      l,
		cache:    c,
		storage:  s,
	}, nil
}

func (i *Inspector) Inspect3rdParty(tmplSrc string, vFn func() ([]*version.Version, error), fields []StructField) error {
	vers, err := vFn()
	if err != nil {
		return err
	}

	goPath, err := i.storage.getGo(defaultGoVersion)
	if err != nil {
		return err
	}

	for _, v := range vers {
		i.manifests = append(i.manifests, manifest{
			TmplSrc: tmplSrc,
			GoVer:   defaultGoVersion,
			GoPath:  goPath,
			AppVer:  v,
			Fields:  fields,
		})
	}
	return nil
}

func (i *Inspector) InspectStdlib(tmplSrc string, fields []StructField) error {
	for _, v := range goVersions {
		goPath, err := i.storage.getGo(v.Original())
		if err != nil {
			return err
		}

		i.manifests = append(i.manifests, manifest{
			TmplSrc: tmplSrc,
			GoVer:   v.Original(),
			GoPath:  goPath,
			AppVer:  v,
			Fields:  fields,
		})
	}
	return nil
}

type result struct {
	manifest manifest
	sfos     []structFieldOffset
}

func (i *Inspector) Do(ctx context.Context) (*schema.TrackedOffsets, error) {
	g, ctx := errgroup.WithContext(ctx)
	todo := make(chan manifest)

	g.Go(func() error {
		defer close(todo)
		for _, m := range i.manifests {
			select {
			case todo <- m:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	c := make(chan []structFieldOffset)
	for n := 0; n < i.NWorkers; n++ {
		g.Go(func() error {
			for m := range todo {
				out, err := i.do(m)
				if err != nil {
					return err
				}

				select {
				case c <- out:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(c)
	}()

	var results []structFieldOffset
	for r := range c {
		results = append(results, r...)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	i.logResults(results)
	return trackedOffsets(results), nil
}

func (i *Inspector) do(m manifest) ([]structFieldOffset, error) {
	var out []structFieldOffset

	uncached := m.Fields[:0] // Use the same backing array.
	for _, f := range m.Fields {
		if sfo, ok := i.cached(m.AppVer, f); ok {
			out = append(out, sfo)
		} else {
			uncached = append(uncached, f)
		}
	}

	if len(uncached) == 0 {
		return out, nil
	}

	d, err := os.MkdirTemp("", "inspect-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(d)

	i.log.Info("rendering application", "src", m.TmplSrc, "dest", d, "version", m.AppVer)
	data := struct{ Version string }{Version: "v" + m.AppVer.String()}
	if err = render(m.TmplSrc, d, data); err != nil {
		return nil, err
	}

	exec, err := i.build(m.AppVer.Original(), m.GoPath, d)
	if errors.Is(err, errBuild) {
		i.log.Error(err, "skipping offsets", "src", m.TmplSrc, "Go", m.GoVer, "version", m.AppVer)
		for _, f := range uncached {
			// Signal these fields were not found.
			out = append(out, structFieldOffset{
				StructField: f,
				Version:     m.AppVer,
				Offset:      -1,
			})
		}
		return out, nil
	} else if err != nil {
		return nil, err
	}

	for _, f := range uncached {
		sfo, err := i.analyze(m.AppVer, exec, f)
		if err != nil {
			return nil, err
		}
		out = append(out, sfo)
	}

	return out, nil
}

func (i *Inspector) cached(ver *version.Version, sf StructField) (structFieldOffset, bool) {
	sfo := structFieldOffset{StructField: sf, Version: ver}

	var ok bool
	sfo.Offset, ok = i.cache.Get(ver.String(), sf.Package, sf.Struct, sf.Field)
	return sfo, ok
}

func (i *Inspector) analyze(ver *version.Version, exec string, sf StructField) (structFieldOffset, error) {
	i.log.Info("analyzing app binary", "package", sf.Package, "binary", exec, "version", ver)

	elfF, err := elf.Open(exec)
	if err != nil {
		return structFieldOffset{}, err
	}
	defer elfF.Close()

	dwarfData, err := elfF.DWARF()
	if err != nil {
		return structFieldOffset{}, err
	}

	return sf.offset(ver, dwarfData), nil
}

var errBuild = errors.New("failed to build")

func (i *Inspector) build(ver, goBin, dir string) (string, error) {
	goModTidy := goBin + " mod tidy -compat=1.17"
	i.log.Info("running go mod tidy", "dir", dir, "cmd", goModTidy)
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

	app := fmt.Sprintf("app%s", ver)
	build := goBin + " build -o " + app
	i.log.Info("building application", "dir", dir, "cmd", build)
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

	return filepath.Join(dir, app), nil
}

func (i *Inspector) logResults(results []structFieldOffset) {
	for _, r := range results {
		if r.Offset < 0 {
			i.log.Info("offsets not found", "name", r.structName(), "version", r.Version)
		} else {
			i.log.Info("offsets found", "name", r.structName(), "version", r.Version)
		}
	}
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
