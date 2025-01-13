// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package process

import "log/slog"

// Stubs for non-linux systems

type TracedProgram struct{}

func NewTracedProgram(pid int, logger *slog.Logger) (*TracedProgram, error) {
	return nil, nil
}

func (p *TracedProgram) Detach() error {
	return nil
}

func (p *TracedProgram) SetMemLockInfinity() error {
	return nil
}

func (p *TracedProgram) Mmap(length uint64, fd uint64) (uint64, error) {
	return 0, nil
}

func (p *TracedProgram) Madvise(addr uint64, length uint64) error {
	return nil
}

func (p *TracedProgram) Mlock(addr uint64, length uint64) error {
	return nil
}
