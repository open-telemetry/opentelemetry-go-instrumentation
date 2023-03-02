package process

import (
	"debug/elf"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process/ptrace"
	"os"
)

const (
	mapSize = 4096 * 6 * 1024
)

type TargetDetails struct {
	PID               int
	Functions         []*Func
	GoVersion         *version.Version
	Libraries         map[string]string
	AllocationDetails *AllocationDetails
}

type AllocationDetails struct {
	Addr    uint64
	EndAddr uint64
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

func (a *processAnalyzer) remoteMmap(pid int, mapSize uint64) (uint64, error) {
	program, err := ptrace.Trace(pid, log.Logger)
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

	addr, err := a.remoteMmap(pid, mapSize)
	if err != nil {
		log.Logger.Error(err, "Failed to mmap")
		return nil, err
	}
	log.Logger.V(0).Info("mmaped remote memory", "start_addr", fmt.Sprintf("%X", addr),
		"end_addr", fmt.Sprintf("%X", addr+mapSize))

	result.AllocationDetails = &AllocationDetails{
		Addr:    addr,
		EndAddr: addr + mapSize,
	}

	if err != nil {
		return nil, err
	}

	funcs, err := findFunctions(elfF, relevantFuncs)
	if err != nil {
		log.Logger.Error(err, "Failed to find functions")
		return nil, err
	}

	result.Functions = funcs
	return result, nil
}

func findFunctions(elfF *elf.File, relevantFuncs map[string]interface{}) ([]*Func, error) {
	result, err := FindFunctionsUnStripped(elfF, relevantFuncs)
	if err != nil {
		if errors.Is(err, elf.ErrNoSymbols) {
			log.Logger.V(0).Info("No symbols found in binary, trying to find functions using .gosymtab")
			return FindFunctionsStripped(elfF, relevantFuncs)
		}
		return nil, err
	}

	return result, nil
}
