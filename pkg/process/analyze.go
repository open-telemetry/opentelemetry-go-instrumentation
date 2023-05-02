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

package process

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/process/ptrace"
)

const (
	// The concurrent trace & span ID pairs lookup size in bytes. Currently set to 24mb.
	// TODO: Review map size.
	mapSize = 25165824
)

// TargetDetails are the details about a target function.
type TargetDetails struct {
	PID               int
	Functions         []*Func
	GoVersion         *version.Version
	Libraries         map[string]string
	AllocationDetails *AllocationDetails
}

// AllocationDetails are the details about allocated memory.
type AllocationDetails struct {
	StartAddr uint64
	EndAddr   uint64
}

// Func represents a function target.
type Func struct {
	Name          string
	Offset        uint64
	ReturnOffsets []uint64
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

func (a *Analyzer) remoteMmap(pid int, mapSize uint64) (uint64, error) {
	program, err := ptrace.NewTracedProgram(pid, log.Logger)
	if err != nil {
		log.Logger.Error(err, "Failed to attach ptrace", "pid", pid)
		return 0, err
	}

	defer func() {
		log.Logger.V(0).Info("Detaching from process", "pid", pid)
		err := program.Detach()
		if err != nil {
			log.Logger.Error(err, "Failed to detach ptrace", "pid", pid)
		}
	}()
	fd := -1
	addr, err := program.Mmap(mapSize, uint64(fd))
	if err != nil {
		log.Logger.Error(err, "Failed to mmap", "pid", pid)
		return 0, err
	}

	return addr, nil
}

// Analyze returns the target details for an actively running process.
func (a *Analyzer) Analyze(pid int, relevantFuncs map[string]interface{}) (*TargetDetails, error) {
	result := &TargetDetails{
		PID: pid,
	}

	f, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, err
	}

	defer f.Close()
	elfF, err := elf.NewFile(f)
	if err != nil {
		return nil, err
	}

	goVersion, modules, err := a.getModuleDetails(elfF)
	if err != nil {
		return nil, err
	}
	result.GoVersion = goVersion
	result.Libraries = modules

	addr, err := a.remoteMmap(pid, mapSize)
	if err != nil {
		log.Logger.Error(err, "Failed to mmap")
		return nil, err
	}
	log.Logger.V(0).Info("mmaped remote memory", "start_addr", fmt.Sprintf("%X", addr),
		"end_addr", fmt.Sprintf("%X", addr+mapSize))

	result.AllocationDetails = &AllocationDetails{
		StartAddr: addr,
		EndAddr:   addr + mapSize,
	}

	if err != nil {
		return nil, err
	}
	symbols, err := elfF.Symbols()
	if err != nil {
		return nil, err
	}

	for _, f := range symbols {
		if _, exists := relevantFuncs[f.Name]; exists {
			offset, err := getFuncOffset(elfF, f)
			if err != nil {
				return nil, err
			}

			returns, err := findFuncReturns(elfF, f, offset)
			if err != nil {
				log.Logger.V(1).Info("can't find function offset. Skipping", "function", f.Name)
				continue
			}

			log.Logger.V(0).Info("found relevant function for instrumentation",
				"function", f.Name,
				"start", offset,
				"returns", returns)
			function := &Func{
				Name:          f.Name,
				Offset:        offset,
				ReturnOffsets: returns,
			}

			result.Functions = append(result.Functions, function)
		}
	}
	if len(result.Functions) == 0 {
		return nil, errors.New("could not find function offsets for instrumenter")
	}

	return result, nil
}

func getFuncOffset(f *elf.File, symbol elf.Symbol) (uint64, error) {
	var sections []*elf.Section

	for i := range f.Sections {
		if f.Sections[i].Flags == elf.SHF_ALLOC+elf.SHF_EXECINSTR {
			sections = append(sections, f.Sections[i])
		}
	}

	if len(sections) == 0 {
		return 0, fmt.Errorf("function %q not found in file", symbol)
	}

	var execSection *elf.Section
	for m := range sections {
		sectionStart := sections[m].Addr
		sectionEnd := sectionStart + sections[m].Size
		if symbol.Value >= sectionStart && symbol.Value < sectionEnd {
			execSection = sections[m]
			break
		}
	}

	if execSection == nil {
		return 0, errors.New("could not find symbol in executable sections of binary")
	}

	return uint64(symbol.Value - execSection.Addr + execSection.Offset), nil
}

func findFuncReturns(elfFile *elf.File, sym elf.Symbol, functionOffset uint64) ([]uint64, error) {
	textSection := elfFile.Section(".text")
	if textSection == nil {
		return nil, errors.New("could not find .text section in binary")
	}

	lowPC := sym.Value
	highPC := lowPC + sym.Size
	offset := lowPC - textSection.Addr
	buf := make([]byte, int(highPC-lowPC))

	readBytes, err := textSection.ReadAt(buf, int64(offset))
	if err != nil {
		return nil, fmt.Errorf("could not read text section: %w", err)
	}
	data := buf[:readBytes]
	instructionIndices, err := findRetInstructions(data)
	if err != nil {
		return nil, fmt.Errorf("error while scanning instructions: %w", err)
	}

	// Add the function lowPC to each index to obtain the actual locations
	newLocations := make([]uint64, len(instructionIndices))
	for i, instructionIndex := range instructionIndices {
		newLocations[i] = instructionIndex + functionOffset
	}

	return newLocations, nil
}
