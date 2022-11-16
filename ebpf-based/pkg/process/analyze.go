package process

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"golang.org/x/arch/x86/x86asm"
)

type TargetDetails struct {
	PID       int
	Functions []*Func
	GoVersion *version.Version
	Libraries map[string]string
}

type Func struct {
	Name          string
	Offset        uint64
	ReturnOffsets []uint64
}

func (t *TargetDetails) IsRegistersABI() bool {
	regAbiMinVersion, _ := version.NewVersion("1.17")
	return t.GoVersion.GreaterThanOrEqual(regAbiMinVersion)
}

func (t *TargetDetails) GetFunctionOffset(name string) (uint64, error) {
	for _, f := range t.Functions {
		if f.Name == name {
			return f.Offset, nil
		}
	}

	return 0, fmt.Errorf("could not find offset for function %s", name)
}

func (t *TargetDetails) GetFunctionReturns(name string) ([]uint64, error) {
	for _, f := range t.Functions {
		if f.Name == name {
			return f.ReturnOffsets, nil
		}
	}

	return nil, fmt.Errorf("could not find returns for function %s", name)
}

func (a *processAnalyzer) Analyze(pid int, relevantFuncs map[string]interface{}) (*TargetDetails, error) {
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

	var pclndat []byte
	if sec := elfF.Section(".gopclntab"); sec != nil {
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

	for _, f := range symTab.Funcs {

		if _, exists := relevantFuncs[f.Name]; exists {
			start, returns, err := a.findFuncOffset(&f, elfF)
			if err != nil {
				return nil, err
			}

			log.Logger.V(0).Info("found relevant function for instrumentation", "function", f.Name, "returns", len(returns))
			function := &Func{
				Name:          f.Name,
				Offset:        start,
				ReturnOffsets: returns,
			}

			result.Functions = append(result.Functions, function)
		}
	}

	return result, nil
}

func (a *processAnalyzer) findFuncOffset(f *gosym.Func, elfF *elf.File) (uint64, []uint64, error) {
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

			var returns []uint64
			for i := 0; i < int(funcLen); {
				inst, err := x86asm.Decode(data[i:], 64)
				if err != nil {
					log.Logger.Error(err, "error while finding function return")
					return 0, nil, err
				}

				if inst.Op == x86asm.RET {
					returns = append(returns, off+uint64(i))
				}

				i += inst.Len
			}

			return off, returns, nil
		}

	}

	return 0, nil, fmt.Errorf("prog not found")
}
