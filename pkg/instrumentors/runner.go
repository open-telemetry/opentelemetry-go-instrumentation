package instrumentors

import (
	"fmt"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
)

func (m *instrumentorsManager) Run(target *process.TargetDetails) error {
	if len(m.instrumentors) == 0 {
		log.Logger.V(0).Info("there are no avilable instrumentations for target process")
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

func (m *instrumentorsManager) load(target *process.TargetDetails) error {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return err
	}

	exe, err := link.OpenExecutable(fmt.Sprintf("/proc/%d/exe", target.PID))
	if err != nil {
		return err
	}
	ctx := &context.InstrumentorContext{
		TargetDetails: target,
		Executable:    exe,
	}

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

func (m *instrumentorsManager) cleanup() {
	close(m.incomingEvents)
	for _, i := range m.instrumentors {
		i.Close()
	}
}

func (m *instrumentorsManager) Close() {
	m.done <- true
}
