// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/elf"
	"errors"
	"fmt"

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

// Analyze returns the target details for an actively running process.
func (a *Analyzer) Analyze(relevantFuncs map[string]interface{}) (*Info, error) {
	result := &Info{ID: a.id}

	elfF, err := elf.Open(a.id.ExePath())
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	bi, err := a.id.BuildInfo()
	if err != nil {
		return nil, err
	}

	goVersion, err := semver.NewVersion(bi.GoVersion)
	if err != nil {
		return nil, err
	}
	result.GoVersion = goVersion
	result.Modules = make(map[string]*semver.Version, len(bi.Deps)+1)
	for _, dep := range bi.Deps {
		depVersion, err := semver.NewVersion(dep.Version)
		if err != nil {
			a.logger.Error("parsing dependency version", "error", err, "dependency", dep)
			continue
		}
		result.Modules[dep.Path] = depVersion
	}
	result.Modules["std"] = goVersion

	funcs, err := a.findFunctions(elfF, relevantFuncs)
	if err != nil {
		return nil, err
	}
	for _, fn := range funcs {
		a.logger.Debug("found function", "function_name", fn)
	}

	result.Functions = funcs
	if len(result.Functions) == 0 {
		return nil, errors.New("could not find function offsets for instrumenter")
	}

	return result, nil
}

func (a *Analyzer) findFunctions(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*binary.Func, error) {
	result, err := binary.FindFunctionsUnStripped(elfF, relevantFuncs)
	if err != nil {
		if errors.Is(err, elf.ErrNoSymbols) {
			a.logger.Debug("No symbols found in binary, trying to find functions using .gosymtab")
			return binary.FindFunctionsStripped(elfF, relevantFuncs)
		}
		return nil, err
	}

	return result, nil
}
