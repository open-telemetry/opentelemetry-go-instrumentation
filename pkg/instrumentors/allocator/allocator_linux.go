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

package allocator

import (
	"os"

	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/log"
)

// Allocator handles the allocation of the BPF file-system.
type Allocator struct{}

// New returns a new [Allocator].
func New() *Allocator {
	return &Allocator{}
}

// Load loads the BPF file-system.
func (a *Allocator) Load(ctx *context.InstrumentorContext) error {
	logger := log.Logger.WithName("allocator")
	if ctx.TargetDetails.AllocationDetails != nil {
		logger = logger.WithValues(
			"start_addr", ctx.TargetDetails.AllocationDetails.StartAddr,
			"end_addr", ctx.TargetDetails.AllocationDetails.EndAddr)
	}
	logger.V(0).Info("Loading allocator")

	err := a.mountBPFFs()
	if err != nil {
		return err
	}

	return nil
}

func (a *Allocator) mountBPFFs() error {
	_, err := os.Stat(bpffs.BPFFsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(bpffs.BPFFsPath, 0755); err != nil {
			return err
		}
	}

	return unix.Mount(bpffs.BPFFsPath, bpffs.BPFFsPath, "bpf", 0, "")
}
