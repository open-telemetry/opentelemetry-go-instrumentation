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
	"os"
	"runtime"

	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/process/ptrace"
)

// AllocationDetails are the details about allocated memory.
type AllocationDetails struct {
	StartAddr uint64
	EndAddr   uint64
}

// Allocate allocates memory for the instrumented process.
func Allocate(pid int) (*AllocationDetails, error) {
	mapSize := uint64(os.Getpagesize() * runtime.NumCPU() * 8)
	addr, err := remoteAllocate(pid, mapSize)
	if err != nil {
		log.Logger.Error(err, "Failed to mmap")
		return nil, err
	}

	log.Logger.V(0).Info("mmaped remote memory", "start_addr", fmt.Sprintf("%X", addr),
		"end_addr", fmt.Sprintf("%X", addr+mapSize))

	return &AllocationDetails{
		StartAddr: addr,
		EndAddr:   addr + mapSize,
	}, nil
}

func remoteAllocate(pid int, mapSize uint64) (uint64, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	program, err := ptrace.NewTracedProgram(pid, log.Logger)
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

	err = program.Madvise(addr, mapSize)
	if err != nil {
		log.Logger.Error(err, "Failed to madvise", "pid", pid)
		return 0, err
	}

	err = program.Mlock(addr, mapSize)
	if err != nil {
		log.Logger.Error(err, "Failed to mlock", "pid", pid)
		return 0, err
	}

	return addr, nil
}
