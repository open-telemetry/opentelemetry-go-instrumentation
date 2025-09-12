// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"syscall"
)

const syscallInstrSize = 2

func getIP(regs *syscall.PtraceRegs) uintptr {
	panic("ptrace amd64-only function called on 386")
}

func getRegs(pid int, regsout *syscall.PtraceRegs) error {
	panic("ptrace amd64-only function called on 386")
}

func setRegs(pid int, regs *syscall.PtraceRegs) error {
	panic("ptrace amd64-only function called on 386")
}

func (p *tracedProgram) Syscall(number uint64, args ...uint64) (uint64, error) {
	panic("ptrace amd64-only function called on 386")
}
