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

	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/bpffs"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/log"
	"golang.org/x/sys/unix"
)

type Allocator struct{}

func New() *Allocator {
	return &Allocator{}
}

func (a *Allocator) Load(ctx *context.InstrumentorContext) error {
	logger := log.Logger.WithName("allocator")
	logger.V(0).Info("Loading allocator", "start_addr",
		ctx.TargetDetails.AllocationDetails.Addr, "end_addr", ctx.TargetDetails.AllocationDetails.EndAddr)

	err := a.mountBpfFS()
	if err != nil {
		return err
	}

	return nil
}

func (a *Allocator) mountBpfFS() error {
	_, err := os.Stat(bpffs.BpfFsPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(bpffs.BpfFsPath, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return unix.Mount(bpffs.BpfFsPath, bpffs.BpfFsPath, "bpf", 0, "")
}
