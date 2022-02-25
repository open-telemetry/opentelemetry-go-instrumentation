package goroutine

import (
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/inject"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/goroutine/bpffs"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -cc clang bpf ./bpf/tracker.bpf.c -- -I/usr/include/bpf -I$BPF_IMPORT

type Tracker struct {
	bpfObjects *bpfObjects
	uprobe     link.Link
}

func NewTracker() *Tracker {
	return &Tracker{}
}

func (g *Tracker) LibraryName() string {
	return "goroutine_tracker"
}

func (g *Tracker) FuncNames() []string {
	return []string{"runtime.casgstatus"}
}

func (g *Tracker) Load(ctx *context.InstrumentorContext) error {
	err := g.mountBpfFS()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(bpffs.GoRoutinesMapDir, 0755); err != nil {
		return err
	}

	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.InjectStructField{
		{
			VarName:    "goid_pos",
			StructName: "runtime.g",
			Field:      "goid",
		},
	})
	if err != nil {
		return err
	}

	g.bpfObjects = &bpfObjects{}
	err = spec.LoadAndAssign(g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.GoRoutinesMapDir,
		},
	})
	if err != nil {
		return err
	}

	var uprobeObj *ebpf.Program
	if ctx.TargetDetails.IsRegistersABI() {
		uprobeObj = g.bpfObjects.UprobeRuntimeCasgstatusByRegisters
	} else {
		uprobeObj = g.bpfObjects.UprobeRuntimeCasgstatus
	}
	uprobeOffset, err := ctx.TargetDetails.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}
	up, err := ctx.Executable.Uprobe("", uprobeObj, &link.UprobeOptions{
		Offset: uprobeOffset,
	})
	if err != nil {
		return err
	}

	g.uprobe = up
	log.Logger.V(0).Info("goroutine tracker loaded")
	return nil
}

func (g *Tracker) mountBpfFS() error {
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

func (g *Tracker) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("goroutine-tracker")
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		iterator := g.bpfObjects.GoroutinesMap.Iterate()
		for {
			var key uint32
			var val int64
			hasNext := iterator.Next(&key, &val)
			if hasNext {
				logger.V(5).Info("go routine details fetched", "key", key, "value", val)
			} else {
				break
			}
		}
	}
}

func (g *Tracker) Close() {
	log.Logger.V(0).Info("closing goroutine tracker")

	if g.uprobe != nil {
		g.uprobe.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
