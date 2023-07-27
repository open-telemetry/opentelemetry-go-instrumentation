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
//go:build 386 || amd64
// +build 386 amd64

package process

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
