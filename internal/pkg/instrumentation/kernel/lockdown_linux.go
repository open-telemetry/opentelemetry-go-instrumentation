// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package kernel

import (
	"bufio"
	"os"
	"strings"
)

// Injectable for tests.
var lockdownPath = "/sys/kernel/security/lockdown"

func getLockdownMode() LockdownMode {
	// If we can't find the file, assume no lockdown
	if _, err := os.Stat(lockdownPath); err == nil {
		f, err := os.Open(lockdownPath)
		if err != nil {
			return LockdownModeIntegrity
		}

		defer f.Close()
		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			lockdown := scanner.Text()
			if strings.Contains(lockdown, "[none]") {
				return LockdownModeNone
			} else if strings.Contains(lockdown, "[integrity]") {
				return LockdownModeIntegrity
			} else if strings.Contains(lockdown, "[confidentiality]") {
				return LockdownModeConfidentiality
			}
			return LockdownModeOther
		}

		return LockdownModeIntegrity
	}

	return LockdownModeNone
}
