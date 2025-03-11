// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package utils

import (
	"github.com/Masterminds/semver/v3"
)

// Stubs for non-linux systems

func GetLinuxKernelVersion() *semver.Version { return nil }

// KernelLockdown represents different Linux Kernel security lockdown modes.
type KernelLockdown uint8

const (
	// KernelLockdownNone represents the 'none' security lockdown mode.
	KernelLockdownNone KernelLockdown = iota + 1
	// KernelLockdownIntegrity represents the 'integrity' security lockdown mode.
	KernelLockdownIntegrity
	// KernelLockdownConfidentiality represents the 'confidentiality' security lockdown mode.
	KernelLockdownConfidentiality
	// KernelLockdownOther represents an unknown security lockdown mode.
	KernelLockdownOther
)

func KernelLockdownMode() KernelLockdown {
	return 0
}

func GetCPUCount() (uint64, error) {
	return 0, nil
}

func estimateBootTimeOffset() (bootTimeOffset int64, err error) {
	return 0, nil
}
