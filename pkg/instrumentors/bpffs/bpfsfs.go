// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bpffs

import (
	"fmt"
	"os"

	"go.opentelemetry.io/auto/pkg/log"

	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/pkg/process"
)

// BPFFsPath is the system path to the BPF file-system.
const bpfFsPath = "/sys/fs/bpf"

func PathForTargetApplication(target *process.TargetDetails) string {
	return fmt.Sprintf("%s/%d", bpfFsPath, target.PID)
}

func Mount(target *process.TargetDetails) error {
	if !isBPFFSMounted() {
		// Directory does not exist, create it and mount
		if err := os.MkdirAll(bpfFsPath, 0755); err != nil {
			return err
		}

		err := unix.Mount(bpfFsPath, bpfFsPath, "bpf", 0, "")
		if err != nil {
			return err
		}
	}

	return os.Mkdir(PathForTargetApplication(target), 0755)
}

func isBPFFSMounted() bool {
	var stat unix.Statfs_t
	err := unix.Statfs(bpfFsPath, &stat)
	if err != nil {
		log.Logger.Error(err, "failed to statfs bpf filesystem")
		return false
	}

	return stat.Type == unix.BPF_FS_MAGIC
}

func Cleanup(target *process.TargetDetails) error {
	return os.RemoveAll(PathForTargetApplication(target))
}
