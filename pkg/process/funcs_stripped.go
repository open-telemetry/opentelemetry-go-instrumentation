package process

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
)

func FindFunctionsStripped(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*Func, error) {
	var pclndat []byte
	if sec := elfF.Section(".gopclntab"); sec != nil {
		var err error
		pclndat, err = sec.Data()
		if err != nil {
			return nil, err
		}
	}

	sec := elfF.Section(".gosymtab")
	if sec == nil {
		return nil, fmt.Errorf("%s section not found in target binary, make sure this is a Go application", ".gosymtab")
	}
	symTabRaw, err := sec.Data()
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

			log.Logger.V(0).Info("found relevant function for instrumentation", "function", f.Name, "returns", len(returns))
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
	off := f.Value
	for _, prog := range elfF.Progs {
		if prog.Type != elf.PT_LOAD || (prog.Flags&elf.PF_X) == 0 {
			continue
		}

		// For more info on this calculation: stackoverflow.com/a/40249502
		if prog.Vaddr <= f.Value && f.Value < (prog.Vaddr+prog.Memsz) {
			off = f.Value - prog.Vaddr + prog.Off

			funcLen := f.End - f.Entry
			data := make([]byte, funcLen)
			_, err := prog.ReadAt(data, int64(f.Value-prog.Vaddr))
			if err != nil {
				log.Logger.Error(err, "error while finding function return")
				return 0, nil, err
			}

			instructionIndices, err := findRetInstructions(data)
			if err != nil {
				log.Logger.Error(err, "error while finding function returns")
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
