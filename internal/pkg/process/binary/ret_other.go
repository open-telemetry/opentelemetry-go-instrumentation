// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux || !(amd64 || arm64)

package binary

// Stubs for systems without a native implementation.

func findRetInstructions(data []byte) ([]uint64, error) {
	return nil, nil
}
