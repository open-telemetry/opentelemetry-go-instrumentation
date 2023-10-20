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
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Allocator handles the allocation of the BPF file-system.
type Allocator struct{}

// New returns a new [Allocator].
func New() *Allocator {
	return &Allocator{}
}

// Load loads the BPF file-system.
func (a *Allocator) Load(target *process.TargetDetails) error {
	logger := log.Logger.WithName("allocator")
	if target.AllocationDetails != nil {
		logger = logger.WithValues(
			"start_addr", target.AllocationDetails.StartAddr,
			"end_addr", target.AllocationDetails.EndAddr)
	}
	logger.V(0).Info("Loading allocator")

	err := bpffs.Mount(target)
	if err != nil {
		return err
	}

	return nil
}

// Cleanup the BPF file-system.
func (a *Allocator) Clean(target *process.TargetDetails) error {
	return bpffs.Cleanup(target)
}
