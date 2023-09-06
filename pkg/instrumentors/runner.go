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

package instrumentors

import (
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/process"
)

// Run runs the event processing loop for all managed Instrumentors.
func (m *Manager) Run(target *process.TargetDetails) error {
	if len(m.instrumentors) == 0 {
		log.Logger.V(0).Info("there are no available instrumentations for target process")
		return nil
	}

	err := m.load(target)
	if err != nil {
		return err
	}

	for _, i := range m.instrumentors {
		go i.Run(m.incomingEvents)
	}

	for {
		select {
		case <-m.done:
			log.Logger.V(0).Info("shutting down all instrumentors due to signal")
			m.cleanup()
			return nil
		case e := <-m.incomingEvents:
			m.otelController.Trace(e)
		}
	}
}

func (m *Manager) load(target *process.TargetDetails) error {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return err
	}

	injector, err := inject.New(target)
	if err != nil {
		return err
	}

	exe, err := link.OpenExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}
	ctx := &context.InstrumentorContext{
		TargetDetails: target,
		Executable:    exe,
		Injector:      injector,
	}

	if err := m.allocator.Load(ctx); err != nil {
		log.Logger.Error(err, "failed to load allocator")
		return err
	}

	// Load instrumentors
	for name, i := range m.instrumentors {
		log.Logger.V(0).Info("loading instrumentor", "name", name)
		err := i.Load(ctx)
		if err != nil {
			log.Logger.Error(err, "error while loading instrumentors, cleaning up", "name", name)
			m.cleanup()
			return err
		}
	}

	log.Logger.V(0).Info("loaded instrumentors to memory", "total_instrumentors", len(m.instrumentors))
	return nil
}

func (m *Manager) cleanup() {
	close(m.incomingEvents)
	for _, i := range m.instrumentors {
		i.Close()
	}
}

// Close closes m.
func (m *Manager) Close() {
	m.done <- true
}
