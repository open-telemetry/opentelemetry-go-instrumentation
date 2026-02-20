// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package binary

import (
	"debug/elf"
	"errors"
	"fmt"
	"math"
)

func FindFunctionsUnStripped(
	elfF *elf.File,
	relevantFuncs map[string]any,
) ([]*Func, error) {
	symbols, err := elfF.Symbols()
	if err != nil {
		return nil, err
	}

	var result []*Func
	for _, f := range symbols {
		_, exists := relevantFuncs[f.Name]
		if !exists {
			continue
		}
		offset, err := getFuncOffsetUnstripped(elfF, f)
		if err != nil {
			return nil, err
		}

		returns, err := findFuncReturnsUnstripped(elfF, f, offset)
		if err != nil {
			return nil, err
		}

		function := &Func{
			Name:          f.Name,
			Offset:        offset,
			ReturnOffsets: returns,
		}

		result = append(result, function)
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
		return 0, fmt.Errorf("function %q not found in file", symbol.Name)
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

	return symbol.Value - execSection.Addr + execSection.Offset, nil
}

func findFuncReturnsUnstripped(
	elfFile *elf.File,
	sym elf.Symbol,
	functionOffset uint64,
) ([]uint64, error) {
	textSection := elfFile.Section(".text")
	if textSection == nil {
		return nil, errors.New("could not find .text section in binary")
	}

	lowPC := sym.Value
	if textSection.Addr > lowPC {
		return nil, fmt.Errorf(
			"invalid .text section address: %d (symbol value %d)",
			textSection.Addr,
			lowPC,
		)
	}
	offset := lowPC - textSection.Addr
	if offset > math.MaxInt64 {
		return nil, fmt.Errorf("invalid offset: %d", offset)
	}

	highPC := lowPC + sym.Size
	buf := make([]byte, highPC-lowPC)

	readBytes, err := textSection.ReadAt(buf, int64(offset)) //nolint:gosec  // Bounds checked.
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
