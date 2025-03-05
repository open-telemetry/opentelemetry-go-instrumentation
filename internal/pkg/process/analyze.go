// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/elf"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

// Info are the details about a target process.
type Info struct {
	ID         ID
	Functions  []*binary.Func
	GoVersion  *semver.Version
	Modules    map[string]*semver.Version
	Allocation *Allocation
}

// NewInfo returns a new Info with information about the process identified by
// id. The functions of the returned Info are filtered by relevantFuncs.
//
// A partial Info and error may be returned for dependencies that cannot be
// parsed.
func NewInfo(id ID, relevantFuncs map[string]interface{}) (*Info, error) {
	elfF, err := elf.Open(id.ExePath())
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	bi, err := id.BuildInfo()
	if err != nil {
		return nil, err
	}

	goVersion, err := semver.NewVersion(bi.GoVersion)
	if err != nil {
		return nil, err
	}

	result := &Info{
		ID:        id,
		GoVersion: goVersion,
	}

	result.Functions, err = findFunctions(elfF, relevantFuncs)
	if err != nil {
		return nil, err
	}

	result.Modules, err = findModules(goVersion, bi.Deps)
	return result, err
}

func findModules(goVer *semver.Version, deps []*debug.Module) (map[string]*semver.Version, error) {
	var err error
	out := make(map[string]*semver.Version, len(deps)+1)
	for _, dep := range deps {
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

func findFunctions(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*binary.Func, error) {
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
