// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/elf"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

// Info are the details about a target process.
type Info struct {
	ID        ID
	Functions []*binary.Func
	GoVersion *semver.Version
	Modules   map[string]*semver.Version

	allocOnce onceResult[*Allocation]
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
	return i.allocOnce.Do(func() (*Allocation, error) {
		return allocate(logger, i.ID)
	})
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

// onceResult is an object that will perform exactly one action if that action
// does not error. For errors, no state is stored and subsequent attempts will
// be tried.
type onceResult[T any] struct {
	done  atomic.Bool
	mutex sync.Mutex
	val   T
}

// Do runs f only once, and only stores the result if f returns a nil error.
// Subsequent calls to Do will return the stored value or they will re-attempt
// to run f and store the result if an error had been returned.
func (o *onceResult[T]) Do(f func() (T, error)) (T, error) {
	if o.done.Load() {
		o.mutex.Lock()
		defer o.mutex.Unlock()
		return o.val, nil
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.done.Load() {
		return o.val, nil
	}

	var err error
	o.val, err = f()
	if err == nil {
		o.done.Store(true)
	}
	return o.val, err
}
