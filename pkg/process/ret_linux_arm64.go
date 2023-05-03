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
//go:build arm64
// +build arm64

package process

import (
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	// In ARM64 each instruction is 4 bytes in length
	armInstructionSize = 4
)

func findRetInstructions(data []byte) ([]uint64, error) {
	var returnOffsets []uint64
	index := 0
	for index < len(data) {
		instruction, err := arm64asm.Decode(data[index:])
		if err == nil && instruction.Op == arm64asm.RET {
			returnOffsets = append(returnOffsets, uint64(index))
		}

		index += armInstructionSize
	}

	return returnOffsets, nil
}
