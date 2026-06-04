// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package bpffs

import "go.opentelemetry.io/auto/internal/pkg/process"

// Stubs for non-linux systems

func PathForTargetApplication(target *process.Info) string {
	return ""
}

func Mount(target *process.Info) error {
	return nil
}

func Cleanup(target *process.Info) error {
	return nil
}
