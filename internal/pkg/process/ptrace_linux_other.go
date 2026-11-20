// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && !amd64 && !arm64

package process

import (
	"errors"
	"runtime"
	"syscall"
)

// Stubs for architectures without a native implementation.

var errUnsupportedArch = errors.New("ptrace: unsupported architecture " + runtime.GOARCH)

const syscallInstrSize = 0

func getIP(*syscall.PtraceRegs) uintptr { return 0 }

func getRegs(int, *syscall.PtraceRegs) error { return errUnsupportedArch }

func setRegs(int, *syscall.PtraceRegs) error { return errUnsupportedArch }

// Syscall runs a syscall at main thread of process.
func (*tracedProgram) Syscall(uint64, ...uint64) (uint64, error) {
	return 0, errUnsupportedArch
}
