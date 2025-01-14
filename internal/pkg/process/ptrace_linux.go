// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

const waitPidErrorMessage = "waitpid ret value: %d"

const (
	// MADV_POPULATE_READ.
	MadvisePopulateRead = 0x16
	// MADV_POPULATE_WRITE.
	MadvisePopulateWrite = 0x17
)

var threadRetryLimit = 10

// tracedProgram is a program to be traced with ptrace.
type tracedProgram struct {
	pid  int
	tids []int

	backupRegs *syscall.PtraceRegs
	backupCode []byte

	logger *slog.Logger
}

// Pid return the pid of traced program.
func (p *tracedProgram) Pid() int {
	return p.pid
}

func waitPid(pid int) error {
	ret, err := unix.Wait4(pid, nil, unix.WALL, nil)
	if err != nil {
		return err
	}
	if ret == pid {
		return nil
	}

	return errors.Errorf(waitPidErrorMessage, ret)
}

// newTracedProgram ptrace all threads of a process.
func newTracedProgram(pid int, logger *slog.Logger) (*tracedProgram, error) {
	tidMap := make(map[int]bool)
	retryCount := make(map[int]int)

	// iterate over the thread group, until it doesn't change
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
					logger.Debug("retry attaching thread", "tid", tid, "retryCount", retryCount[tid], "limit", threadRetryLimit)
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
					logger.Error("detach failed", "error", e, "tid", tid)
				}
				return nil, errors.WithStack(err)
			}

			logger.Debug("attach successfully", "tid", tid)
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

	program := &tracedProgram{
		pid:        pid,
		tids:       tids,
		backupRegs: &syscall.PtraceRegs{},
		backupCode: make([]byte, syscallInstrSize),
		logger:     logger,
	}

	return program, nil
}

// Detach detaches from all threads of the processes.
func (p *tracedProgram) Detach() error {
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
func (p *tracedProgram) Protect() error {
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
func (p *tracedProgram) Restore() error {
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
func (p *tracedProgram) Wait() error {
	_, err := syscall.Wait4(p.pid, nil, 0, nil)
	return err
}

// Step moves one step forward.
func (p *tracedProgram) Step() error {
	err := syscall.PtraceSingleStep(p.pid)
	if err != nil {
		return errors.WithStack(err)
	}

	return p.Wait()
}

// SetMemLockInfinity sets the memlock rlimit to infinity.
func (p *tracedProgram) SetMemLockInfinity() error {
	// Requires CAP_SYS_RESOURCE.
	newLimit := unix.Rlimit{Cur: unix.RLIM_INFINITY, Max: unix.RLIM_INFINITY}
	if err := unix.Prlimit(p.pid, unix.RLIMIT_MEMLOCK, &newLimit, nil); err != nil {
		return fmt.Errorf("failed to set memlock rlimit: %w", err)
	}

	return nil
}

// Mmap runs mmap syscall.
func (p *tracedProgram) Mmap(length uint64, fd uint64) (uint64, error) {
	return p.Syscall(syscall.SYS_MMAP, 0, length, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE|syscall.MAP_POPULATE|syscall.MAP_LOCKED, fd, 0)
}

// Madvise runs madvise syscall.
func (p *tracedProgram) Madvise(addr uint64, length uint64) error {
	advice := uint64(syscall.MADV_WILLNEED)
	ver, err := utils.GetLinuxKernelVersion()
	if err != nil {
		return errors.WithStack(err)
	}

	minVersion := version.Must(version.NewVersion("5.14"))
	p.logger.Debug("Detected linux kernel version", "version", ver)
	if ver.GreaterThanOrEqual(minVersion) {
		advice = syscall.MADV_WILLNEED | MadvisePopulateRead | MadvisePopulateWrite
	}

	_, err = p.Syscall(syscall.SYS_MADVISE, addr, length, advice, 0, 0, 0)
	return err
}

// Mlock runs mlock syscall.
func (p *tracedProgram) Mlock(addr uint64, length uint64) error {
	ret, err := p.Syscall(syscall.SYS_MLOCK, addr, length, 0, 0, 0, 0)
	p.logger.Debug("mlock ret", "ret", ret)
	return err
}
