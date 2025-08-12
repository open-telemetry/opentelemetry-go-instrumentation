// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build 386 || amd64
// +build 386 amd64

package binary

import (
	"fmt"

	"golang.org/x/arch/x86/x86asm"
)

func findRetInstructions(data []byte) ([]uint64, error) {
	nInt := len(data)
	if nInt < 0 {
		return nil, fmt.Errorf("invalid data length: %d", nInt)
	}
	n := uint64(nInt)

	var returnOffsets []uint64
	var index uint64
	for index < n {
		instruction, err := x86asm.Decode(data[index:], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode x64 instruction at offset %d: %w", index, err)
		}

		if instruction.Op == x86asm.RET {
			returnOffsets = append(returnOffsets, index)
		}

		index += uint64(max(0, instruction.Len)) //nolint:gosec  // Underflow handled.
	}

	return returnOffsets, nil
}
