// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/buildinfo"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

// TargetDetails are the details about a target function.
type TargetDetails struct {
	PID               int
	Functions         []*binary.Func
	GoVersion         *version.Version
	Modules           map[string]*version.Version
	AllocationDetails *AllocationDetails
}

// IsRegistersABI returns if t is supported.
func (t *TargetDetails) IsRegistersABI() bool {
	regAbiMinVersion, _ := version.NewVersion("1.17")
	return t.GoVersion.GreaterThanOrEqual(regAbiMinVersion)
}

// GetFunctionOffset returns the offset for of the function with name.
func (t *TargetDetails) GetFunctionOffset(name string) (uint64, error) {
	for _, f := range t.Functions {
		if f.Name == name {
			return f.Offset, nil
		}
	}

	return 0, fmt.Errorf("could not find offset for function %s", name)
}

// GetFunctionReturns returns the return value of the call for the function
// with name.
func (t *TargetDetails) GetFunctionReturns(name string) ([]uint64, error) {
	for _, f := range t.Functions {
		if f.Name == name {
			return f.ReturnOffsets, nil
		}
	}

	return nil, fmt.Errorf("could not find returns for function %s", name)
}

// OpenExe opens the executable of the target process for reading.
func (t *TargetDetails) OpenExe() (*os.File, error) {
	path := fmt.Sprintf("/proc/%d/exe", t.PID)
	return os.Open(path)
}

// Analyze returns the target details for an actively running process.
func (a *Analyzer) Analyze(pid int, relevantFuncs map[string]interface{}) (*TargetDetails, error) {
	result := &TargetDetails{PID: pid}

	f, err := result.OpenExe()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	elfF, err := elf.NewFile(f)
	if err != nil {
		return nil, err
	}

	goVersion, err := version.NewVersion(a.BuildInfo.GoVersion)
	if err != nil {
		return nil, err
	}
	result.GoVersion = goVersion
	result.Modules = make(map[string]*version.Version, len(a.BuildInfo.Deps)+1)
	for _, dep := range a.BuildInfo.Deps {
		depVersion, err := version.NewVersion(dep.Version)
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

func (a *Analyzer) SetBuildInfo(pid int) error {
	f, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return err
	}

	defer f.Close()
	bi, err := buildinfo.Read(f)
	if err != nil {
		return err
	}

	bi.GoVersion = parseGoVersion(bi.GoVersion)

	a.BuildInfo = bi
	return nil
}

func parseGoVersion(vers string) string {
	vers = strings.ReplaceAll(vers, "go", "")
	// Trims GOEXPERIMENT version suffix if present.
	if idx := strings.Index(vers, " X:"); idx > 0 {
		vers = vers[:idx]
	}
	return vers
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
