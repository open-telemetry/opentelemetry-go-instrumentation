// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"runtime"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

// Allocation represent memory that has been allocated for a process.
type Allocation struct {
	StartAddr uint64
	EndAddr   uint64
	NumCPU    uint64
}

// allocate allocates memory for the instrumented process.
func allocate(logger *slog.Logger, id ID) (*Allocation, error) {
	// runtime.NumCPU doesn't query any kind of hardware or OS state,
	// but merely uses affinity APIs to count what CPUs the given go process is available to run on.
	// Go's implementation of runtime.NumCPU (https://github.com/golang/go/blob/48d899dcdbed4534ed942f7ec2917cf86b18af22/src/runtime/os_linux.go#L97)
	// uses sched_getaffinity to count the number of CPUs the process is allowed to run on.
	// We are interested in the number of CPUs available to the system.
	nCPU, err := utils.GetCPUCount()
	if err != nil {
		return nil, err
	}

	n := os.Getpagesize()
	if n < 0 {
		return nil, fmt.Errorf("invalid page size: %d", n)
	}
	pagesize := uint64(n) // nolint: gosec  // Bound checked.

	mapSize := pagesize * nCPU * 8
	logger.Debug(
		"Requesting memory allocation",
		"size", mapSize,
		"page size", pagesize,
		"cpu count", nCPU)

	addr, err := remoteAllocate(logger, id, mapSize)
	if err != nil {
		return nil, err
	}

	logger.Debug(
		"mmaped remote memory",
		"start_addr", fmt.Sprintf("0x%x", addr),
		"end_addr", fmt.Sprintf("0x%x", addr+mapSize),
	)

	return &Allocation{
		StartAddr: addr,
		EndAddr:   addr + mapSize,
		NumCPU:    nCPU,
	}, nil
}

func remoteAllocate(logger *slog.Logger, id ID, mapSize uint64) (uint64, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	program, err := newTracedProgram(id, logger)
	if err != nil {
		return 0, err
	}

	defer func() {
		logger.Info("Detaching from process", "pid", id)
		err := program.Detach()
		if err != nil {
			logger.Error("Failed to detach ptrace", "error", err, "pid", id)
		}
	}()

	if err := program.SetMemLockInfinity(); err != nil {
		logger.Error("Failed to set memlock on process", "error", err)
	} else {
		logger.Debug("Set memlock on process successfully")
	}

	addr, err := program.Mmap(mapSize, math.MaxUint64)
	if err != nil {
		return 0, err
	}
	if addr == math.MaxUint64 {
		// On success, mmap() returns a pointer to the mapped area.
		// On error, the value MAP_FAILED (that is, (void *) -1) is returned
		return 0, errors.New("mmap MAP_FAILED")
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
