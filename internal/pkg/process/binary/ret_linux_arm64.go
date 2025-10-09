// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build arm64

package binary

import (
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

const (
	// In ARM64 each instruction is 4 bytes in length.
	armInstructionSize = 4
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
		instruction, err := arm64asm.Decode(data[index:])
		if err == nil && instruction.Op == arm64asm.RET {
			returnOffsets = append(returnOffsets, index)
		}

		index += armInstructionSize
	}

	return returnOffsets, nil
}
