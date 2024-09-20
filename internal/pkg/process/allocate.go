// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"runtime"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process/ptrace"
)

// AllocationDetails are the details about allocated memory.
type AllocationDetails struct {
	StartAddr uint64
	EndAddr   uint64
	NumCPU    uint64
}

// Allocate allocates memory for the instrumented process.
func Allocate(logger *slog.Logger, pid int) (*AllocationDetails, error) {
	// runtime.NumCPU doesn't query any kind of hardware or OS state,
	// but merely uses affinity APIs to count what CPUs the given go process is available to run on.
	// Go's implementation of runtime.NumCPU (https://github.com/golang/go/blob/48d899dcdbed4534ed942f7ec2917cf86b18af22/src/runtime/os_linux.go#L97)
	// uses sched_getaffinity to count the number of CPUs the process is allowed to run on.
	// We are interested in the number of CPUs available to the system.
	nCPU, err := utils.GetCPUCount()
	if err != nil {
		return nil, err
	}

	mapSize := uint64(os.Getpagesize() * nCPU * 8)
	logger.Debug(
		"Requesting memory allocation",
		"size", mapSize,
		"page size", os.Getpagesize(),
		"cpu count", nCPU)

	addr, err := remoteAllocate(logger, pid, mapSize)
	if err != nil {
		return nil, err
	}

	logger.Debug(
		"mmaped remote memory",
		"start_addr", fmt.Sprintf("0x%x", addr),
		"end_addr", fmt.Sprintf("0x%x", addr+mapSize),
	)

	return &AllocationDetails{
		StartAddr: addr,
		EndAddr:   addr + mapSize,
		NumCPU:    uint64(nCPU),
	}, nil
}

func remoteAllocate(logger *slog.Logger, pid int, mapSize uint64) (uint64, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	program, err := ptrace.NewTracedProgram(pid, logger)
	if err != nil {
		return 0, err
	}

	defer func() {
		logger.Info("Detaching from process", "pid", pid)
		err := program.Detach()
		if err != nil {
			logger.Error("Failed to detach ptrace", "error", err, "pid", pid)
		}
	}()

	if err := program.SetMemLockInfinity(); err != nil {
		logger.Error("Failed to set memlock on process", "error", err)
	} else {
		logger.Debug("Set memlock on process successfully")
	}

	fd := -1
	addr, err := program.Mmap(mapSize, uint64(fd))
	if err != nil {
		return 0, err
	}
	if addr == math.MaxUint64 {
		// On success, mmap() returns a pointer to the mapped area.
		// On error, the value MAP_FAILED (that is, (void *) -1) is returned
		return 0, fmt.Errorf("mmap MAP_FAILED")
	}

	err = program.Madvise(addr, mapSize)
	if err != nil {
		return 0, err
	}

	err = program.Mlock(addr, mapSize)
	if err != nil {
		return 0, err
	}

	return addr, nil
}
