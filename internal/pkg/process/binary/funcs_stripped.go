// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package binary

import (
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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

	// we extract the `textStart` value based on the header of the pclntab,
	// this is used to parse the line number table, and is not necessarily the start of the `.text` section.
	// when a binary is build with C code, the value of `textStart` is not the same as the start of the `.text` section.
	// https://github.com/golang/go/blob/master/src/runtime/symtab.go#L374
	var runtimeText uint64
	ptrSize := uint32(pclndat[7])
	if ptrSize == 4 {
		runtimeText = uint64(binary.LittleEndian.Uint32(pclndat[8+2*ptrSize:]))
	} else {
		runtimeText = binary.LittleEndian.Uint64(pclndat[8+2*ptrSize:])
	}

	pcln := gosym.NewLineTable(pclndat, runtimeText)
	symTab, err := gosym.NewTable(nil, pcln)
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
	text := elfF.Section(".text")
	if text == nil {
		return 0, nil, errors.New(".text section not found in target binary")
	}

	var off uint64
	funcLen := max(f.End-f.Entry, 0)
	data := make([]byte, funcLen)
	offInText := f.Entry - text.Addr

	if offInText > math.MaxInt64 {
		return 0, nil, fmt.Errorf("overflow in offset to read in the text section: %d", offInText)
	}

	_, err := text.ReadAt(data, int64(offInText)) // nolint: gosec // Overflow handled.
	if err != nil {
		return 0, nil, err
	}

	retInstructionOffsets, err := findRetInstructions(data)
	if err != nil {
		return 0, nil, err
	}

	for _, prog := range elfF.Progs {
		if prog.Type != elf.PT_LOAD || (prog.Flags&elf.PF_X) == 0 {
			continue
		}

		// For more info on this calculation: stackoverflow.com/a/40249502
		if prog.Vaddr <= f.Value && f.Value < (prog.Vaddr+prog.Memsz) {
			off = f.Value - prog.Vaddr + prog.Off
			break
		}
	}

	if off == 0 {
		return 0, nil, errors.New("could not find function offset")
	}

	retOffsets := make([]uint64, len(retInstructionOffsets))
	for i, instructionOffset := range retInstructionOffsets {
		retOffsets[i] = instructionOffset + off
	}

	return off, retOffsets, nil
}
