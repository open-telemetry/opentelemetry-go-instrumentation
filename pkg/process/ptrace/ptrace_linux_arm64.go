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
	"encoding/binary"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var endian = binary.LittleEndian

const syscallInstrSize = 4

// see kernel source /include/uapi/linux/elf.h.
const nrPRStatus = 1

func getIP(regs *syscall.PtraceRegs) uintptr {
	return uintptr(regs.Pc)
}

func getRegs(pid int, regsout *syscall.PtraceRegs) error {
	err := unix.PtraceGetRegSetArm64(pid, nrPRStatus, (*unix.PtraceRegsArm64)(regsout))
	if err != nil {
		return errors.Wrapf(err, "get registers of process %d", pid)
	}
	return nil
}

func setRegs(pid int, regs *syscall.PtraceRegs) error {
	err := unix.PtraceSetRegSetArm64(pid, nrPRStatus, (*unix.PtraceRegsArm64)(regs))
	if err != nil {
		return errors.Wrapf(err, "set registers of process %d", pid)
	}
	return nil
}

// Syscall runs a syscall at main thread of process.
func (p *TracedProgram) Syscall(number uint64, args ...uint64) (uint64, error) {
	if len(args) > 7 {
		return 0, errors.New("too many arguments for a syscall")
	}

	// save the original registers and the current instructions
	err := p.Protect()
	if err != nil {
		return 0, err
	}

	var regs syscall.PtraceRegs

	err = getRegs(p.pid, &regs)
	if err != nil {
		return 0, err
	}
	// set the registers according to the syscall convention. Learn more about
	// it in `man 2 syscall`. In aarch64 the syscall nr is stored in w8, and the
	// arguments are stored in x0, x1, x2, x3, x4, x5 in order
	regs.Regs[8] = number
	copy(regs.Regs[:len(args)], args)

	err = setRegs(p.pid, &regs)
	if err != nil {
		return 0, err
	}

	instruction := make([]byte, syscallInstrSize)
	ip := getIP(p.backupRegs)

	// most aarch64 devices are little endian
	// 0xd4000001 is `svc #0` to call the system call
	endian.PutUint32(instruction, 0xd4000001)
	_, err = syscall.PtracePokeData(p.pid, ip, instruction)
	if err != nil {
		return 0, errors.Wrapf(err, "writing data %v to %x", instruction, ip)
	}

	// run one instruction, and stop
	err = p.Step()
	if err != nil {
		return 0, err
	}

	// read registers, the return value of syscall is stored inside x0 register
	err = getRegs(p.pid, &regs)
	if err != nil {
		return 0, err
	}

	return regs.Regs[0], p.Restore()
}
