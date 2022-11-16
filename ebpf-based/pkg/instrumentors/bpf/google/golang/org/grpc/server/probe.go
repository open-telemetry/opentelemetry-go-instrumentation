package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/inject"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/goroutine/bpffs"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

type GrpcEvent struct {
	GoRoutine int64
	StartTime uint64
	EndTime   uint64
	Method    [100]byte
}

type grpcServerInstrumentor struct {
	bpfObjects   *bpfObjects
	uprobe       link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

func New() *grpcServerInstrumentor {
	return &grpcServerInstrumentor{}
}

func (g *grpcServerInstrumentor) LibraryName() string {
	return "google.golang.org/grpc/server"
}

func (g *grpcServerInstrumentor) FuncNames() []string {
	return []string{"google.golang.org/grpc.(*Server).handleStream"}
}

func (g *grpcServerInstrumentor) Load(ctx *context.InstrumentorContext) error {
	targetLib := "google.golang.org/grpc"
	libVersion, exists := ctx.TargetDetails.Libraries[targetLib]
	if !exists {
		libVersion = ""
	}
	spec, err := ctx.Injector.Inject(loadBpf, "google.golang.org/grpc", libVersion, []*inject.InjectStructField{
		{
			VarName:    "stream_method_ptr_pos",
			StructName: "google.golang.org/grpc/internal/transport.Stream",
			Field:      "method",
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

	offset, err := ctx.TargetDetails.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}

	var uprobeObj *ebpf.Program
	if ctx.TargetDetails.IsRegistersABI() {
		uprobeObj = g.bpfObjects.UprobeServerHandleStreamByRegisters
	} else {
		uprobeObj = g.bpfObjects.UprobeServerHandleStream
	}

	up, err := ctx.Executable.Uprobe("", uprobeObj, &link.UprobeOptions{
		Offset: offset,
	})
	if err != nil {
		return err
	}

	g.uprobe = up
	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(g.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeServerHandleStreamReturns, &link.UprobeOptions{
			Offset: ret,
		})
		if err != nil {
			return err
		}
		g.returnProbs = append(g.returnProbs, retProbe)
	}

	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd

	return nil
}

func (g *grpcServerInstrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("grpc-server-instrumentor")
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

func (g *grpcServerInstrumentor) convertEvent(e *GrpcEvent) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])

	return &events.Event{
		Library:      g.LibraryName(),
		GoroutineUID: e.GoRoutine,
		Name:         method,
		Kind:         trace.SpanKindServer,
		StartTime:    int64(e.StartTime),
		EndTime:      int64(e.EndTime),
		Attributes: []attribute.KeyValue{
			semconv.RPCSystemKey.String("grpc"),
			semconv.RPCServiceKey.String(method),
		},
	}
}

func (g *grpcServerInstrumentor) Close() {
	log.Logger.V(0).Info("closing gRPC server instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	if g.uprobe != nil {
		g.uprobe.Close()
	}

	for _, r := range g.returnProbs {
		r.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
