package server

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	"os"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -cc clang bpf ./bpf/probe.bpf.c -- -I/usr/include/bpf -I$BPF_IMPORT

type HttpEvent struct {
	GoRoutine int64
	Method    [100]byte
	Path      [100]byte
}

type httpServerInstrumentor struct {
	bpfObjects   *bpfObjects
	uprobe       link.Link
	eventsReader *perf.Reader
}

func New() *httpServerInstrumentor {
	return &httpServerInstrumentor{}
}

func (h *httpServerInstrumentor) LibraryName() string {
	return "net/http"
}

func (h *httpServerInstrumentor) FuncNames() []string {
	return []string{"net/http.(*ServeMux).ServeHTTP"}
}

func (h *httpServerInstrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.InjectStructField{
		{
			VarName:    "method_ptr_pos",
			StructName: "net/http.Request",
			Field:      "Method",
		},
		{
			VarName:    "url_ptr_pos",
			StructName: "net/http.Request",
			Field:      "URL",
		},
		{
			VarName:    "path_ptr_pos",
			StructName: "net/url.URL",
			Field:      "Path",
		},
	})

	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}
	err = spec.LoadAndAssign(h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.GoRoutinesMapDir,
		},
	})
	if err != nil {
		return err
	}

	offset, err := ctx.TargetDetails.GetFunctionOffset(h.FuncNames()[0])
	if err != nil {
		return err
	}

	var uprobeObj *ebpf.Program
	if ctx.TargetDetails.IsRegistersABI() {
		uprobeObj = h.bpfObjects.UprobeServerMuxServeHTTP_ByRegisters
	} else {
		uprobeObj = h.bpfObjects.UprobeServerMuxServeHTTP
	}

	up, err := ctx.Executable.Uprobe("", uprobeObj, &link.UprobeOptions{
		Offset: offset,
	})
	if err != nil {
		return err
	}

	h.uprobe = up
	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	h.eventsReader = rd

	return nil
}

func (h *httpServerInstrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("net/http-instrumentor")
	var event HttpEvent
	for {
		record, err := h.eventsReader.Read()
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

		eventsChan <- h.convertEvent(&event)
	}
}

func (h *httpServerInstrumentor) convertEvent(e *HttpEvent) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])

	return &events.Event{
		Library:      h.LibraryName(),
		GoroutineUID: e.GoRoutine,
		Name:         path,
		Kind:         trace.SpanKindServer,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
		},
	}
}

func (h *httpServerInstrumentor) Close() {
	log.Logger.V(0).Info("closing net/http instrumentor")
	if h.eventsReader != nil {
		h.eventsReader.Close()
	}

	if h.uprobe != nil {
		h.uprobe.Close()
	}

	if h.bpfObjects != nil {
		h.bpfObjects.Close()
	}
}
