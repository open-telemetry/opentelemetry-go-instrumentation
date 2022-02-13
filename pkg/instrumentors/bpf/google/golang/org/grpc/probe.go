package grpc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/goroutine/bpffs"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
	"os"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -cc clang bpf ./bpf/probe.bpf.c -- -I/usr/include/bpf -I$BPF_IMPORT

type GrpcEvent struct {
	GoRoutine uint64
	Method    [100]byte
	Target    [100]byte
}

type grpcInstrumentor struct {
	bpfObjects   *bpfObjects
	uprobe       link.Link
	eventsReader *perf.Reader
}

func New() *grpcInstrumentor {
	return &grpcInstrumentor{}
}

func (g *grpcInstrumentor) LibraryName() string {
	return "google.golang.org/grpc"
}

func (g *grpcInstrumentor) FuncNames() []string {
	return []string{"google.golang.org/grpc.(*ClientConn).Invoke"}
}

func (g *grpcInstrumentor) Load(ctx *context.InstrumentorContext) error {
	g.bpfObjects = &bpfObjects{}
	err := loadBpfObjects(g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.GoRoutinesMapDir,
		},
	})
	if err != nil {
		return err
	}

	offset, err := ctx.TargetDetails.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}

	var uprobeObj *ebpf.Program
	if ctx.TargetDetails.RegistersABI {
		uprobeObj = g.bpfObjects.UprobeClientConnInvokeByRegisters
	} else {
		uprobeObj = g.bpfObjects.UprobeClientConnInvoke
	}
	up, err := ctx.Executable.Uprobe("", uprobeObj, &link.UprobeOptions{
		Offset: offset,
	})
	if err != nil {
		return err
	}

	g.uprobe = up
	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd

	return nil
}

func (g *grpcInstrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("grpc-instrumentor")
	var event GrpcEvent
	for {
		record, err := g.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			logger.Error(err, "error parsing perf event")
			continue
		}

		eventsChan <- g.convertEvent(&event)
	}
}

// According to https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func (g *grpcInstrumentor) convertEvent(e *GrpcEvent) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	target := unix.ByteSliceToString(e.Target[:])

	return &events.Event{
		Library:      g.LibraryName(),
		GoroutineUID: e.GoRoutine,
		Name:         method,
		Kind:         trace.SpanKindClient,
		Attributes: []attribute.KeyValue{
			semconv.RPCSystemKey.String("grpc"),
			semconv.RPCServiceKey.String(method),
			semconv.NetPeerIPKey.String(target),
			semconv.NetPeerNameKey.String(target),
		},
	}
}

func (g *grpcInstrumentor) Close() {
	log.Logger.V(0).Info("closing gRPC instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	if g.uprobe != nil {
		g.uprobe.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
