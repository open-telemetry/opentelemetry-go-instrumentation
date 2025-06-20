// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package kernel

import (
	"syscall"

	"github.com/Masterminds/semver/v3"
)

// unameFn is allows testing with a mock for syscall.Uname.
var unameFn = syscall.Uname

func version() *semver.Version {
	// Adapted from https://github.com/golang/go/blob/go1.21.3/src/internal/syscall/unix/kernel_version_linux.go

	var uname syscall.Utsname
	if err := unameFn(&uname); err != nil {
		return nil
	}

	var (
		values [2]uint64
		value  uint64
		vi     int
	)
	for _, c := range uname.Release {
		if '0' <= c && c <= '9' {
			value = (value * 10) + uint64(c-'0') // nolint:gosec  // c >= '0'
		} else {
			// Note that we're assuming N.N.N here.
			// If we see anything else, we are likely to mis-parse it.
			values[vi] = value
			vi++
			if vi >= len(values) {
				break
			}
			value = 0
		}
	}
	return semver.New(values[0], values[1], 0, "", "")
}
