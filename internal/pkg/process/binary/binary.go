// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package binary provides types and functionality to handle function
// definitions within a target Go binary.
package binary

// Func represents a function target.
type Func struct {
	Name          string
	Offset        uint64
	ReturnOffsets []uint64
}
