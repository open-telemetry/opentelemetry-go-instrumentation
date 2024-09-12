// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package utils

import "github.com/hashicorp/go-version"

// Stubs for non-linux systems

func GetLinuxKernelVersion() (*version.Version, error) {
	return &version.Version{}, nil
}

type KernelLockdown uint8

const (
	KernelLockdownNone            KernelLockdown = iota + 1 // Linux Kernel security lockdown mode [none]
	KernelLockdownIntegrity                                 // Linux Kernel security lockdown mode [integrity]
	KernelLockdownConfidentiality                           // Linux Kernel security lockdown mode [confidentiality]
	KernelLockdownOther                                     // Linux Kernel security lockdown mode unknown
)

func KernelLockdownMode() KernelLockdown {
	return 0
}

func GetCPUCount() (int, error) {
	return 0, nil
}

func estimateBootTimeOffset() (bootTimeOffset int64, err error) {
	return 0, nil
}
