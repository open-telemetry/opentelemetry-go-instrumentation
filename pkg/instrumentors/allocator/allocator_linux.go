package allocator

import (
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpffs"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"golang.org/x/sys/unix"
	"os"
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
