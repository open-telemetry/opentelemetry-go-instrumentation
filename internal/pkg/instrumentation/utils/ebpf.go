// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"os"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

const (
	showVerifierLogEnvVar = "OTEL_GO_AUTO_SHOW_VERIFIER_LOG"
)

// ShouldShowVerifierLogs returns if the user has configured verifier logs to be emitted.
func ShouldShowVerifierLogs() bool {
	val, exists := os.LookupEnv(showVerifierLogEnvVar)
	if exists {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}

// SupportsContextPropagation returns if the Linux kernel supports use of
// bpf_probe_write_user. It will check for supported versions of the Linux
// kernel and then verify if /sys/kernel/security/lockdown is not locked down.
func SupportsContextPropagation() bool {
	ver := GetLinuxKernelVersion()
	if ver == nil {
		return false
	}

	noLockKernel := semver.New(5, 14, 0, "", "")
	if ver.LessThan(noLockKernel) {
		return true
	}

	return KernelLockdownMode() == KernelLockdownNone
}
