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
	"errors"
	"fmt"
)

func FindFunctionsUnStripped(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*Func, error) {
	symbols, err := elfF.Symbols()
	if err != nil {
		return nil, err
	}

	var result []*Func
	for _, f := range symbols {
		if _, exists := relevantFuncs[f.Name]; exists {
			offset, err := getFuncOffsetUnstripped(elfF, f)
			if err != nil {
				return nil, err
			}

			returns, err := findFuncReturnsUnstripped(elfF, f, offset)
			if err != nil {
				return nil, err
			}

			logFoundFunction(f.Name, offset, returns)
			function := &Func{
				Name:          f.Name,
				Offset:        offset,
				ReturnOffsets: returns,
			}

			result = append(result, function)
		}
	}

	return result, nil
}

func getFuncOffsetUnstripped(f *elf.File, symbol elf.Symbol) (uint64, error) {
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

func findFuncReturnsUnstripped(elfFile *elf.File, sym elf.Symbol, functionOffset uint64) ([]uint64, error) {
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
