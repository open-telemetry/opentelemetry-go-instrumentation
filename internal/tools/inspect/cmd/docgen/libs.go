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
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors"
	"go.opentelemetry.io/auto/offsets-tracker/utils"
	"go.opentelemetry.io/auto/offsets-tracker/versions"
	"golang.org/x/mod/semver"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/database/sql"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/github.com/gin-gonic/gin"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc"
	grpcserver "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/google.golang.org/grpc/server"
	httpclient "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/client"
	httpserver "go.opentelemetry.io/auto/internal/pkg/instrumentors/bpf/net/http/server"
)

// Packages are the instrumented packages and the versions supported.
var Packages []Package

type Package struct {
	pkg

	Versions []VersionRange
}

type VersionRange struct {
	Min string
	Max string
}

var (
	goVersions = func() []string {
		minGo := version.MustConstraints(version.NewConstraint(">= 1.12"))

		found, err := versions.FindVersionsFromGoWebsite()
		if err != nil {
			panic(err)
		}

		var vers []string
		for _, v := range found {
			parsed, err := version.NewVersion(v)
			if err != nil {
				panic(err)
			}

			if minGo.Check(parsed) {
				vers = append(vers, v)
			}
		}
		return vers
	}()

	base = []pkg{
		{
			Name:     "net/http",
			PkgGoURL: "https://pkg.go.dev/net/http",

			src: "templates/net/http/*.tmpl",
			instrs: []instrumentors.Instrumentor{
				httpclient.New(),
				httpserver.New(),
			},
			verFunc: func() ([]string, error) { return goVersions, nil },
		},
		{
			Name:     "google.golang.org/grpc",
			PkgGoURL: "https://pkg.go.dev/google.golang.org/grpc",

			src: "templates/google.golang.org/grpc/*.tmpl",
			instrs: []instrumentors.Instrumentor{
				grpc.New(),
				grpcserver.New(),
			},
			verFunc: func() ([]string, error) {
				return versions.FindVersionsUsingGoList("google.golang.org/grpc")
			},
		},
		{
			Name:     "github.com/gin-gonic/gin",
			PkgGoURL: "https://pkg.go.dev/github.com/gin-gonic/gin",

			src:    "templates/github.com/gin-gonic/gin/*.tmpl",
			instrs: []instrumentors.Instrumentor{gin.New()},
			verFunc: func() ([]string, error) {
				return versions.FindVersionsUsingGoList("github.com/gin-gonic/gin")
			},
		},
		{
			Name:     "database/sql",
			PkgGoURL: "https://pkg.go.dev/database/sql",

			src:     "templates/database/sql/*.tmpl",
			instrs:  []instrumentors.Instrumentor{sql.New()},
			verFunc: func() ([]string, error) { return goVersions, nil },
		},
	}
)

type pkg struct {
	Name     string
	PkgGoURL string

	// src is the test app template source.
	src string
	// verFunc returns all package versions.
	verFunc func() ([]string, error)
	// instrs are the providers of instrumentation.
	instrs []instrumentors.Instrumentor
}

func (p pkg) init() (Package, error) {
	v, err := p.versions()
	return Package{pkg: p, Versions: v}, err
}

func (p pkg) versions() ([]VersionRange, error) {
	fmt.Println("Getting versions for", p.Name)
	versions, err := p.verFunc()
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}
	sortVersions(versions)

	ranges := []VersionRange{{}}
	for i, v := range versions {
		err := p.check(v)
		if err == nil {
			if ranges[len(ranges)-1].Min == "" {
				ranges[len(ranges)-1].Min = v
			}
			fmt.Println("Version", v, "supported for", p.Name)
		} else if errors.Is(err, errMissing) {
			if ranges[len(ranges)-1].Min != "" {
				ranges[len(ranges)-1].Max = versions[i-1]
				ranges = append(ranges, VersionRange{})
			}
			fmt.Println("Version", v, "not supported for", p.Name, ":", err)
		} else if errors.Is(err, errFailBuild) {
			if ranges[len(ranges)-1].Min != "" {
				ranges[len(ranges)-1].Max = versions[i-1]
				ranges = append(ranges, VersionRange{})
			}
			fmt.Println("Version", v, "failed to build for", p.Name, ":", err)
		} else {
			return nil, err
		}
	}

	switch {
	case ranges[len(ranges)-1].Min == "":
		// Finished with a range of unsupported versions.
		ranges = ranges[:len(ranges)-1]
	case ranges[len(ranges)-1].Max == "":
		// Supported last version.
		ranges[len(ranges)-1].Max = versions[len(versions)-1]
	}

	return ranges, nil
}

var (
	errMissing   = errors.New("missing subprogram")
	errFailBuild = errors.New("failed to build")
)

func (p pkg) check(version string) error {
	dir, err := os.MkdirTemp("", "docgen*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	render(p.src, dir, struct {
		Version    string
		MajorMinor string
	}{
		Version:    version,
		MajorMinor: popV(semver.MajorMinor(preV(version))),
	})

	_, stderr, err := utils.RunCommand("go mod tidy -compat=1.17", dir)
	if err != nil {
		log.Println(stderr)
		return err
	}
	_, _, err = utils.RunCommand("go build -o app", dir)
	if err != nil {
		// If it cannot compile, it does not support this version.
		return errFailBuild
	}

	var funcNames []string
	for _, instr := range p.instrs {
		funcNames = append(funcNames, instr.FuncNames()...)
	}

	execPath := filepath.Join(dir, "app")
	var missing []string
	for _, name := range funcNames {
		has, err := hasSubprogram(execPath, name)
		if err != nil {
			return err
		}
		if !has {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %v", errMissing, missing)
	}
	return nil
}

func init() {
	Packages = make([]Package, 0, len(base))

	// Ordered docs.
	sort.Slice(base, func(i, j int) bool { return base[i].Name < base[j].Name })
	for _, p := range base {
		pkg, err := p.init()
		if err != nil {
			log.Fatal(p.Name, err)
		}
		Packages = append(Packages, pkg)
	}
}

/*
func stdlibVersByField(pkg, structName, field string) func() (string, string) {
	vc := version.MustConstraints(version.NewConstraint(">= " + minGoVer))
	dm := []*binary.StructField{{
		StructName: pkg + "." + structName,
		Field:      field,
	}}

	return func() (string, string) {
		data := target.New(pkg, offsets, true)
		data = data.FindVersionsBy(target.GoDevFileVersionsStrategy)
		data = data.VersionConstraint(&vc)
		r, err := data.FindOffsets(dm)
		if err != nil {
			panic(err)
		}
		return resultsMinMax(r)
	}
}

func versByField(pkg, structName, field string) func() (string, string) {
	dm := []*binary.StructField{{
		StructName: pkg + "." + structName,
		Field:      field,
	}}

	return func() (string, string) {
		r, err := target.New(pkg, offsets, false).FindOffsets(dm)
		if err != nil {
			panic(err)
		}
		return resultsMinMax(r)
	}
}

func versByFunc(pkg, structName, name string) func() (string, string) {
	ff := []*binary.FuncField{{
		Name:       name,
		Package:    pkg,
		StructName: structName,
	}}

	return func() (string, string) {
		r, err := target.New(pkg, offsets, false).FindFuncOffsets(ff)
		if err != nil {
			panic(err)
		}
		return resultsMinMax(r)
	}
}

func resultsMinMax(r *target.Result) (min, max string) {
	if len(r.ResultsByVersion) < 2 {
		msg := fmt.Sprintf("invalid module versions: %s", r.ModuleName)
		panic(msg)
	}
	sort.Slice(r.ResultsByVersion, func(i, j int) bool {
		verI := r.ResultsByVersion[i].Version
		verJ := r.ResultsByVersion[j].Version
		return semver.Compare(verI, verJ) < 0
	})

	max = r.ResultsByVersion[0].Version
	min = r.ResultsByVersion[len(r.ResultsByVersion)-1].Version
	return min, max
}
*/

func hasSubprogram(target, name string) (bool, error) {
	elfFile, err := elf.Open(target)
	if err != nil {
		return false, err
	}

	dwarfData, err := elfFile.DWARF()
	if err != nil {
		return false, err
	}

	reader := dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err == io.EOF || entry == nil {
			break
		} else if err != nil {
			return false, err
		}

		if entry.Tag == dwarf.TagSubprogram {
			// Determine if this is what we are looking for
			for _, field := range entry.Field {
				if field.Attr == dwarf.AttrName {
					v := field.Val.(string)
					if v == name {
						return true, nil
					}
				}
			}
		}
	}
	return false, nil
}

func preV(s string) string {
	if s == "" {
		return "v"
	}
	if s[0] != 'v' {
		return "v" + s
	}
	return s
}

func popV(s string) string {
	if s == "" {
		return s
	}
	if s[0] == 'v' {
		return s[1:]
	}
	return s
}

func sortVersions(v []string) {
	sort.Slice(v, func(i, j int) bool {
		vI, vJ := preV(v[i]), preV(v[j])
		cmp := semver.Compare(vI, vJ)
		if cmp != 0 {
			return cmp < 0
		}
		return vI < vJ
	})
}
