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

package binary

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
)

func FindFunctionsStripped(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*Func, error) {
	var sec *elf.Section
	if sec = elfF.Section(".gopclntab"); sec == nil {
		return nil, fmt.Errorf("%s section not found in target binary", ".gopclntab")
	}
	pclndat, err := sec.Data()
	if err != nil {
		return nil, err
	}
	sec = elfF.Section(".gosymtab")
	if sec == nil {
		return nil, fmt.Errorf("%s section not found in target binary, make sure this is a Go application", ".gosymtab")
	}
	symTabRaw, err := sec.Data()
	if err != nil {
		return nil, err
	}
	pcln := gosym.NewLineTable(pclndat, elfF.Section(".text").Addr)
	symTab, err := gosym.NewTable(symTabRaw, pcln)
	if err != nil {
		return nil, err
	}

	var result []*Func
	for _, f := range symTab.Funcs {
		if _, exists := relevantFuncs[f.Name]; exists {
			start, returns, err := findFuncOffsetStripped(&f, elfF)
			if err != nil {
				return nil, err
			}

			function := &Func{
				Name:          f.Name,
				Offset:        start,
				ReturnOffsets: returns,
			}

			result = append(result, function)
		}
	}

	return result, nil
}

func findFuncOffsetStripped(f *gosym.Func, elfF *elf.File) (uint64, []uint64, error) {
	for _, prog := range elfF.Progs {
		if prog.Type != elf.PT_LOAD || (prog.Flags&elf.PF_X) == 0 {
			continue
		}

		// For more info on this calculation: stackoverflow.com/a/40249502
		if prog.Vaddr <= f.Value && f.Value < (prog.Vaddr+prog.Memsz) {
			off := f.Value - prog.Vaddr + prog.Off

			funcLen := f.End - f.Entry
			data := make([]byte, funcLen)
			_, err := prog.ReadAt(data, int64(f.Value-prog.Vaddr))
			if err != nil {
				return 0, nil, err
			}

			instructionIndices, err := findRetInstructions(data)
			if err != nil {
				return 0, nil, err
			}

			newLocations := make([]uint64, len(instructionIndices))
			for i, instructionIndex := range instructionIndices {
				newLocations[i] = instructionIndex + off
			}

			return off, newLocations, nil
		}
	}
	return 0, nil, fmt.Errorf("prog not found")
}
