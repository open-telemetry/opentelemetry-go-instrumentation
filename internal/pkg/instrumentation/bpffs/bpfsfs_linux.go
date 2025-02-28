// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bpffs

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/process"
)

// BPFFsPath is the system path to the BPF file-system.
const bpfFsPath = "/sys/fs/bpf"

// PathForTargetApplication returns the path to the BPF file-system for the given target.
func PathForTargetApplication(target *process.Info) string {
	return fmt.Sprintf("%s/%d", bpfFsPath, target.PID)
}

// Mount mounts the BPF file-system for the given target.
func Mount(target *process.Info) error {
	if !isBPFFSMounted() {
		// Directory does not exist, create it and mount
		if err := os.MkdirAll(bpfFsPath, 0o755); err != nil {
			return err
		}

		err := unix.Mount(bpfFsPath, bpfFsPath, "bpf", 0, "")
		if err != nil {
			return err
		}
	}

	// create directory with read, write and execute permissions
	return os.Mkdir(PathForTargetApplication(target), 0o755)
}

func isBPFFSMounted() bool {
	var stat unix.Statfs_t
	err := unix.Statfs(bpfFsPath, &stat)
	if err != nil {
		return false
	}

	return stat.Type == unix.BPF_FS_MAGIC
}

// Cleanup removes the BPF file-system for the given target.
func Cleanup(target *process.Info) error {
	return os.RemoveAll(PathForTargetApplication(target))
}
