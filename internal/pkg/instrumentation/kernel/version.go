// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

import "github.com/Masterminds/semver/v3"

// Version returns the current version of the kernel. If unable to determine
// the function, nil is returned.
func Version() *semver.Version { return version() }
