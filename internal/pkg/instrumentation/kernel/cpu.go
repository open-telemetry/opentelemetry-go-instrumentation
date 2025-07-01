// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

// GetCPUCount returns the number of CPUs available on the system.
func GetCPUCount() (uint64, error) { return cpuCount() }
