// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/elf"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

// Info are the details about a target process.
type Info struct {
	ID        ID
	Functions []*binary.Func
	// GoVersion is the semantic version of Go run by the target process.
	//
	// Experimental and build information included in the version is dropped.
	// If a development version of Go is used, the commit hash will be included
	// in the metadata of the version.
	GoVersion *semver.Version
	Modules   map[string]*semver.Version

	aDone atomic.Bool
	aMu   sync.Mutex
	a     *Allocation
}

// Alloc allocates memory for the process described by Info i.
//
// The underlying memory allocation is only successfully performed once for the
// instance i. Meaning, it is safe to call this multiple times. The first
// successful result will be returned to all subsequent calls. If an error is
// returned, subsequent calls will re-attempt to perform the allocation.
//
// It is safe to call this method concurrently.
func (i *Info) Alloc(logger *slog.Logger) (*Allocation, error) {
	if !i.aDone.Load() {
		// Outlined complex-path to allow inlining here.
		return i.alloc(logger)
	}
	return i.a, nil
}

// Used for testing.
var allocateFn = allocate

func (i *Info) alloc(logger *slog.Logger) (*Allocation, error) {
	i.aMu.Lock()
	defer i.aMu.Unlock()

	if !i.aDone.Load() {
		a, err := allocateFn(logger, i.ID)
		if err != nil {
			return a, err
		}
		i.a = a
		i.aDone.Store(true)
	}
	return i.a, nil
}

// NewInfo returns a new Info with information about the process identified by
// id. The functions of the returned Info are filtered by relevantFuncs.
//
// A partial Info and error may be returned for dependencies that cannot be
// parsed.
func NewInfo(id ID, relevantFuncs map[string]any) (*Info, error) {
	elfF, err := elf.Open(id.ExePath())
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	result := &Info{ID: id}

	bi, err := id.BuildInfo()
	if err != nil {
		return nil, err
	}

	result.GoVersion, err = goVer(bi.GoVersion)
	if err != nil {
		return nil, err
	}

	result.Functions, err = findFunctions(elfF, relevantFuncs)
	if err != nil {
		return result, err
	}

	result.Modules, err = findModules(result.GoVersion, bi.Deps)
	return result, err
}

func goVer(raw string) (*semver.Version, error) {
	if strings.HasPrefix(raw, "devel") {
		return goDevVer(raw)
	}
	raw = strings.TrimPrefix(raw, "go")

	// Handle local modified versions of Go.
	raw = strings.TrimSuffix(raw, "+")

	// Trims GOEXPERIMENT version suffix if present.
	if idx := strings.Index(raw, " X:"); idx > 0 {
		raw = raw[:idx]
	}

	return semver.NewVersion(raw)
}

var devVerRE = regexp.MustCompile(`devel \+([a-f0-9]+) `)

func goDevVer(raw string) (*semver.Version, error) {
	// Parse development versions. For example,
	//    "devel +8e496f1 Thu Nov 5 15:41:05 2015 +0000"

	matches := devVerRE.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return semver.New(0, 0, 0, "", matches[1]), nil
	}
	return nil, errors.New("non-devel version")
}

// develModVer is the version string used for development versions of modules.
// It is used to indicate that the module is not a released version, but rather
// a development version that may include uncommitted changes or is in an
// unstable state.
//
// https://github.com/open-telemetry/opentelemetry-go-instrumentation/issues/2388
const develModVer = "(devel)"

// VerDevel is the placeholder version used for modules that use the
// development version "(devel)".
var VerDevel = semver.MustParse("0.0.0-dev")

func findModules(goVer *semver.Version, deps []*debug.Module) (map[string]*semver.Version, error) {
	var err error
	out := make(map[string]*semver.Version, len(deps)+1)
	for _, dep := range deps {
		if dep.Version == develModVer {
			// dep.Version is not a parsable semantic version. Do not error.
			// Instead, use VerDevel to signal this development version state.
			out[dep.Path] = VerDevel
			continue
		}

		depVersion, e := semver.NewVersion(dep.Version)
		if e != nil {
			err = errors.Join(
				err,
				fmt.Errorf("invalid dependency version %s (%s): %w", dep.Path, dep.Version, e),
			)
			continue
		}
		out[dep.Path] = depVersion
	}
	out["std"] = goVer
	return out, err
}

func findFunctions(elfF *elf.File, relevantFuncs map[string]any) ([]*binary.Func, error) {
	found, err := binary.FindFunctionsUnStripped(elfF, relevantFuncs)
	if err != nil {
		if !errors.Is(err, elf.ErrNoSymbols) {
			return nil, err
		}
		found, err = binary.FindFunctionsStripped(elfF, relevantFuncs)
		if err != nil {
			return nil, err
		}
	}

	if len(found) == 0 {
		return nil, errors.New("no functions found")
	}

	return found, nil
}

// GetFunctionOffset returns the offset for of the function with name.
func (i *Info) GetFunctionOffset(name string) (uint64, error) {
	for _, f := range i.Functions {
		if f.Name == name {
			return f.Offset, nil
		}
	}

	return 0, fmt.Errorf("could not find offset for function %s", name)
}

// GetFunctionReturns returns the return value of the call for the function
// with name.
func (i *Info) GetFunctionReturns(name string) ([]uint64, error) {
	for _, f := range i.Functions {
		if f.Name == name {
			return f.ReturnOffsets, nil
		}
	}

	return nil, fmt.Errorf("could not find returns for function %s", name)
}
