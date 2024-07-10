// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package binary

// Stubs for non-linux systems

func findRetInstructions(data []byte) ([]uint64, error) {
	return nil, nil
}
