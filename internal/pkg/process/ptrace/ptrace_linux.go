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

package ptrace

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const waitPidErrorMessage = "waitpid ret value: %d"

var threadRetryLimit = 10

// TracedProgram is a program traced by ptrace.
type TracedProgram struct {
	pid  int
	tids []int

	backupRegs *syscall.PtraceRegs
	backupCode []byte

	logger logr.Logger
}

// Pid return the pid of traced program.
func (p *TracedProgram) Pid() int {
	return p.pid
}

func waitPid(pid int) error {
	ret := waitpid(pid)
	if ret == pid {
		return nil
	}

	return errors.Errorf(waitPidErrorMessage, ret)
}

// NewTracedProgram ptrace all threads of a process.
func NewTracedProgram(pid int, logger logr.Logger) (*TracedProgram, error) {
	tidMap := make(map[int]bool)
	retryCount := make(map[int]int)

	// iterate over the thread group, until it doens't change
	//
	// we have tried several ways to ensure that we have stopped all the tasks:
	// 1. iterating over and over again to make sure all of them are tracee
	// 2. send `SIGSTOP` signal
	// ...
	// only the first way finally worked for every situations
	for {
		threads, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// judge whether `threads` is a subset of `tidMap`
		subset := true

		tids := make(map[int]bool)
		for _, thread := range threads {
			tid64, err := strconv.ParseInt(thread.Name(), 10, 32)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			tid := int(tid64)

			_, ok := tidMap[tid]
			if ok {
				tids[tid] = true
				continue
			}
			subset = false

			err = syscall.PtraceAttach(tid)
			if err != nil {
				_, ok := retryCount[tid]
				if !ok {
					retryCount[tid] = 1
				} else {
					retryCount[tid]++
				}
				if retryCount[tid] < threadRetryLimit {
					logger.Info("retry attaching thread", "tid", tid, "retryCount", retryCount[tid], "limit", threadRetryLimit)
					continue
				}

				if !strings.Contains(err.Error(), "no such process") {
					return nil, errors.WithStack(err)
				}
				continue
			}

			err = waitPid(tid)
			if err != nil {
				e := syscall.PtraceDetach(tid)
				if e != nil && !strings.Contains(e.Error(), "no such process") {
					logger.Error(e, "detach failed", "tid", tid)
				}
				return nil, errors.WithStack(err)
			}

			logger.Info("attach successfully", "tid", tid)
			tids[tid] = true
			tidMap[tid] = true
		}

		if subset {
			tidMap = tids
			break
		}
	}

	var tids []int
	for key := range tidMap {
		tids = append(tids, key)
	}

	program := &TracedProgram{
		pid:        pid,
		tids:       tids,
		backupRegs: &syscall.PtraceRegs{},
		backupCode: make([]byte, syscallInstrSize),
		logger:     logger,
	}

	return program, nil
}

// Detach detaches from all threads of the processes.
func (p *TracedProgram) Detach() error {
	for _, tid := range p.tids {
		err := syscall.PtraceDetach(tid)

		if err != nil {
			if !strings.Contains(err.Error(), "no such process") {
				return errors.WithStack(err)
			}
		}
	}

	return nil
}

// Protect will backup regs and rip into fields.
func (p *TracedProgram) Protect() error {
	err := getRegs(p.pid, p.backupRegs)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = syscall.PtracePeekData(p.pid, getIP(p.backupRegs), p.backupCode)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Restore will restore regs and rip from fields.
func (p *TracedProgram) Restore() error {
	err := setRegs(p.pid, p.backupRegs)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = syscall.PtracePokeData(p.pid, getIP(p.backupRegs), p.backupCode)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Wait waits until the process stops.
func (p *TracedProgram) Wait() error {
	_, err := syscall.Wait4(p.pid, nil, 0, nil)
	return err
}

// Step moves one step forward.
func (p *TracedProgram) Step() error {
	err := syscall.PtraceSingleStep(p.pid)
	if err != nil {
		return errors.WithStack(err)
	}

	return p.Wait()
}

// Mmap runs mmap syscall.
func (p *TracedProgram) Mmap(length uint64, fd uint64) (uint64, error) {
	return p.Syscall(syscall.SYS_MMAP, 0, length, syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC, syscall.MAP_ANON|syscall.MAP_PRIVATE|syscall.MAP_POPULATE, fd, 0)
}
