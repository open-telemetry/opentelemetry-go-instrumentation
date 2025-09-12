// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build 386
// +build 386

package binary

import (
	"fmt"

	"golang.org/x/arch/x86/x86asm"
)

func findRetInstructions(data []byte) ([]uint64, error) {
	n := len(data)
	if n < 0 {
		return nil, fmt.Errorf("invalid data length: %d", n)
	}

	var returnOffsets []uint64
	var index int
	for index < n {
		instruction, err := x86asm.Decode(data[index:], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode x64 instruction at offset %d: %w", index, err)
		}

		if instruction.Op == x86asm.RET {
			returnOffsets = append(returnOffsets, uint64(index))
		}

		index += max(0, instruction.Len)
	}

	return returnOffsets, nil
}
