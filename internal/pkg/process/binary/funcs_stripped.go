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

func FindFunctionsStripped(elfF *elf.File, relevantFuncs map[string]any) ([]*Func, error) {
	var sec *elf.Section
	if sec = elfF.Section(".gopclntab"); sec == nil {
		return nil, fmt.Errorf("%s section not found in target binary", ".gopclntab")
	}
	pclndat, err := sec.Data()
	if err != nil {
		return nil, err
	}

	// We need to read the Go pcln data at offset 8 + 2 * the pointer size.
	// The pointer size can be found at offset 7, which should be either 4 or 8.
	// We assume that we shouldn't have a gopclntab section smaller than the
	// 8 + 2 * the largest possible pointer size, which is 8 + 2 * 8.
	if len(pclndat) <= 8*2*8 {
		return nil, errors.New(".gopclntab section too small")
	}

	ptrSize := uint32(pclndat[7])
	if ptrSize != 4 && ptrSize != 8 {
		return nil, errors.New("invalid pointer size of text section of .gopclntab")
	}

	runtimeText, err := textStart(elfF, sec, pclndat, ptrSize)
	if err != nil {
		return nil, err
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

	_, err := text.ReadAt(data, int64(offInText)) //nolint:gosec // Overflow handled.
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

// textStart returns the address of runtime.text, the start of the Go text
// segment, for the target binary. This is the base address of the function
// entry offsets stored in the pclntab. It is not necessarily the start of the
// .text section: binaries linked with C code place the C runtime before the Go
// text.
//
// Go versions prior to 1.26 store this address in the pclntab header. Go 1.26
// and later leave that header field zero (it needed a relocation) and only
// store it in runtime.firstmoduledata, which is located by searching the data
// sections for the pointer to the pclntab that starts the moduledata struct.
func textStart(
	elfF *elf.File, pclnSec *elf.Section, pclndat []byte, ptrSize uint32,
) (uint64, error) {
	// https://github.com/golang/go/blob/go1.25.0/src/runtime/symtab.go#L374
	if v := readWord(elfF.ByteOrder, pclndat[8+2*ptrSize:], ptrSize); v != 0 {
		return v, nil
	}

	v, err := moduledataText(elfF, pclnSec.Addr, ptrSize)
	if err != nil {
		return 0, fmt.Errorf("failed to find start of Go text segment: %w", err)
	}
	return v, nil
}

// Word offsets of runtime.moduledata fields, in units of the pointer size.
// The layout of the leading fields has been stable since the pcHeader was
// introduced in Go 1.16:
//
//	pcHeader                                     *pcHeader // 1 word
//	funcnametab, cutab, filetab, pctab, pclntable []T      // 5 x 3 words
//	ftab                                         []functab // 3 words
//	findfunctab                                  uintptr
//	minpc, maxpc                                 uintptr
//	text, etext                                  uintptr
//
// https://github.com/golang/go/blob/go1.26.0/src/runtime/symtab.go
const (
	mdMinPCWord = 20
	mdMaxPCWord = 21
	mdTextWord  = 22
	mdEtextWord = 23
	mdWords     = 24
)

// moduledataText locates runtime.firstmoduledata in the ELF data sections and
// returns its text field. The moduledata struct starts with a pointer to the
// pclntab header, so candidates are found by scanning for pclntabAddr.
// Candidates are validated against the .text section bounds to rule out
// unrelated pointers to the pclntab.
//
// Go 1.26 and later place the moduledata in its own .go.module section; older
// versions keep it in .noptrdata.
func moduledataText(elfF *elf.File, pclntabAddr uint64, ptrSize uint32) (uint64, error) {
	textSec := elfF.Section(".text")
	if textSec == nil {
		return 0, errors.New(".text section not found in target binary")
	}
	textEnd := textSec.Addr + textSec.Size

	stride := int(ptrSize)
	for _, name := range []string{".go.module", ".noptrdata", ".data"} {
		sec := elfF.Section(name)
		if sec == nil {
			continue
		}
		data, err := sec.Data()
		if err != nil {
			return 0, err
		}

		for off := 0; off+mdWords*stride <= len(data); off += stride {
			if readWord(elfF.ByteOrder, data[off:], ptrSize) != pclntabAddr {
				continue
			}

			word := func(i int) uint64 {
				return readWord(elfF.ByteOrder, data[off+i*stride:], ptrSize)
			}
			text, etext := word(mdTextWord), word(mdEtextWord)
			minpc, maxpc := word(mdMinPCWord), word(mdMaxPCWord)

			if text < textSec.Addr || text >= textEnd || etext <= text || etext > textEnd {
				continue
			}
			if minpc < text || maxpc > etext || maxpc < minpc {
				continue
			}
			return text, nil
		}
	}

	return 0, errors.New("runtime.firstmoduledata not found in target binary")
}

// readWord reads a pointer-sized unsigned integer from the start of b.
func readWord(order binary.ByteOrder, b []byte, ptrSize uint32) uint64 {
	if ptrSize == 4 {
		return uint64(order.Uint32(b))
	}
	return order.Uint64(b)
}
