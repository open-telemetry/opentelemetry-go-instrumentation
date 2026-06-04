// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package kernel

func cpuCount() (uint64, error) { return 0, nil }
