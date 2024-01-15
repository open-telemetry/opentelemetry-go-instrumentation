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

package process

import (
	"fmt"
	"math"
	"os"
	"runtime"

	"github.com/go-logr/logr"

	"go.opentelemetry.io/auto/internal/pkg/process/ptrace"
)

// AllocationDetails are the details about allocated memory.
type AllocationDetails struct {
	StartAddr uint64
	EndAddr   uint64
}

// Allocate allocates memory for the instrumented process.
func Allocate(logger logr.Logger, pid int) (*AllocationDetails, error) {
	logger = logger.WithName("Allocate")

	mapSize := uint64(os.Getpagesize() * runtime.NumCPU() * 8)
	addr, err := remoteAllocate(logger, pid, mapSize)
	if err != nil {
		return nil, err
	}

	logger.Info(
		"mmaped remote memory",
		"start_addr", fmt.Sprintf("%X", addr),
		"end_addr", fmt.Sprintf("%X", addr+mapSize),
	)

	return &AllocationDetails{
		StartAddr: addr,
		EndAddr:   addr + mapSize,
	}, nil
}

func remoteAllocate(logger logr.Logger, pid int, mapSize uint64) (uint64, error) {
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
			logger.Error(err, "Failed to detach ptrace", "pid", pid)
		}
	}()

	if err := program.SetMemLockInfinity(); err != nil {
		logger.Error(err, "Failed to set memlock on process")
	} else {
		logger.Info("Set memlock on process successfully")
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
