// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package bpffs

import "go.opentelemetry.io/auto/internal/pkg/process"

// Stubs for non-linux systems

func PathForTargetApplication(target *process.TargetDetails) string {
	return ""
}

func Mount(target *process.TargetDetails) error {
	return nil
}

func Cleanup(target *process.TargetDetails) error {
	return nil
}
