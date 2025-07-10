// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

// LockdownMode is the lockdown state of the Linux kernel.
type LockdownMode uint8

const (
	// LockdownModeNone is the "none" Linux Kernel security lockdown mode.
	LockdownModeNone LockdownMode = iota + 1
	// LockdownModeIntegrity is the "integrity" Linux Kernel security lockdown mode.
	LockdownModeIntegrity
	// LockdownModeConfidentiality is the "confidentiality" Linux Kernel security lockdown mode.
	LockdownModeConfidentiality
	// LockdownModeOther is the "unknown" Linux Kernel security lockdown mode.
	LockdownModeOther
)

// GetLockdownMode returns the current Linux Kernel security lockdown mode.
func GetLockdownMode() LockdownMode { return getLockdownMode() }
