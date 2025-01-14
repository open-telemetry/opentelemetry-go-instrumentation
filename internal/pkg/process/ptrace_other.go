// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package process

import "log/slog"

// Stubs for non-linux systems

type tracedProgram struct{}

func newTracedProgram(pid int, logger *slog.Logger) (*tracedProgram, error) {
	return nil, nil
}

func (p *tracedProgram) Detach() error {
	return nil
}

func (p *tracedProgram) SetMemLockInfinity() error {
	return nil
}

func (p *tracedProgram) Mmap(length uint64, fd uint64) (uint64, error) {
	return 0, nil
}

func (p *tracedProgram) Madvise(addr uint64, length uint64) error {
	return nil
}

func (p *tracedProgram) Mlock(addr uint64, length uint64) error {
	return nil
}
