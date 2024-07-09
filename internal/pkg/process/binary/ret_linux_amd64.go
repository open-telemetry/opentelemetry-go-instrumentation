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
	var returnOffsets []uint64
	index := 0
	for index < len(data) {
		instruction, err := x86asm.Decode(data[index:], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode x64 instruction at offset %d: %w", index, err)
		}

		if instruction.Op == x86asm.RET {
			returnOffsets = append(returnOffsets, uint64(index))
		}

		index += instruction.Len
	}

	return returnOffsets, nil
}
