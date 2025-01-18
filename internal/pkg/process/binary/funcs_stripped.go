// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package binary

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"fmt"
	"strings"
)

// From go/src/debug/gosym/pclntab.go.
const (
	go12magic  = 0xfffffffb
	go116magic = 0xfffffffa
	go118magic = 0xfffffff0
	go120magic = 0xfffffff1
)

func FindFunctionsStripped(elfF *elf.File, relevantFuncs map[string]interface{}, bi *buildinfo.BuildInfo) ([]*Func, error) {
	var sec *elf.Section
	if sec = elfF.Section(".gopclntab"); sec == nil {
		return nil, fmt.Errorf("%s section not found in target binary", ".gopclntab")
	}
	pclndat, err := sec.Data()
	if err != nil {
		return nil, err
	}

	// locate the header of the pclntab, by finding the magic number
	magic := magicNumber(bi.GoVersion)
	pclntabIndex := bytes.Index(pclndat, magic)
	if pclntabIndex < 0 {
		return nil, fmt.Errorf("could not find pclntab magic number")
	}

	pclndat = pclndat[pclntabIndex:]
	// reading the `runtime.text` value from the table,
	// this value may differ from the actual text segment start address
	// in case the binary is compiled with CGO_ENABLED=1
	var runtimeText uint64
	// Get textStart from pclntable
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

// Select the magic number based on the Go version.
func magicNumber(goVersion string) []byte {
	// goVersion here is not the original string returned from BuildInfo.GoVersion
	// which has the form of "go1.17.1". Instead, it is the version number without
	// the "go" prefix, e.g. "1.17.1".
	bs := make([]byte, 4)
	var magic uint32
	if strings.Compare(goVersion, "1.20") >= 0 {
		magic = go120magic
	} else if strings.Compare(goVersion, "1.18") >= 0 {
		magic = go118magic
	} else if strings.Compare(goVersion, "1.16") >= 0 {
		magic = go116magic
	} else {
		magic = go12magic
	}
	binary.LittleEndian.PutUint32(bs, magic)
	return bs
}

func findFuncOffsetStripped(f *gosym.Func, elfF *elf.File) (uint64, []uint64, error) {
	text := elfF.Section(".text")
	if text == nil {
		return 0, nil, fmt.Errorf(".text section not found in target binary")
	}

	var off uint64
	funcLen := f.End - f.Entry
	data := make([]byte, funcLen)

	_, err := text.ReadAt(data, int64(f.Entry-text.Addr))
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
		return 0, nil, fmt.Errorf("could not find function offset")
	}

	retOffsets := make([]uint64, len(retInstructionOffsets))
	for i, instructionOffset := range retInstructionOffsets {
		retOffsets[i] = instructionOffset + off
	}

	return off, retOffsets, nil
}
