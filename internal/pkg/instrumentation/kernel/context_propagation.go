// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

import "github.com/Masterminds/semver/v3"

// lockBPFProbeWriteUserVer is the kernel version that locked down bpf_probe_write_user.
var lockBPFProbeWriteUserVer = semver.New(5, 14, 0, "", "")

// SupportsContextPropagation returns if the Linux kernel supports use of
// bpf_probe_write_user. It will check for supported versions of the Linux
// kernel and then verify if /sys/kernel/security/lockdown is not locked down.
func SupportsContextPropagation() bool {
	ver := Version()
	if ver == nil {
		return false
	}

	if ver.LessThan(lockBPFProbeWriteUserVer) {
		return true
	}

	return GetLockdownMode() == LockdownModeNone
}
