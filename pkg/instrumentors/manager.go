package instrumentors

import (
	"fmt"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/allocator"
	gorillaMux "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/github.com/gorilla/mux"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc"
	grpcServer "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc/server"
	httpServer "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/net/http/server"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/opentelemetry"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
)

var (
	ErrNotAllFuncsFound = fmt.Errorf("not all functions found for instrumentation")
)

type instrumentorsManager struct {
	instrumentors  map[string]Instrumentor
	done           chan bool
	incomingEvents chan *events.Event
	otelController *opentelemetry.Controller
	allocator      *allocator.Allocator
}

func NewManager(otelController *opentelemetry.Controller) (*instrumentorsManager, error) {
	m := &instrumentorsManager{
		instrumentors:  make(map[string]Instrumentor),
		done:           make(chan bool, 1),
		incomingEvents: make(chan *events.Event),
		otelController: otelController,
		allocator:      allocator.New(),
	}

	err := registerInstrumentors(m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *instrumentorsManager) registerInstrumentor(instrumentor Instrumentor) error {
	if _, exists := m.instrumentors[instrumentor.LibraryName()]; exists {
		return fmt.Errorf("library %s registered twice, aborting", instrumentor.LibraryName())
	}

	m.instrumentors[instrumentor.LibraryName()] = instrumentor
	return nil
}

func (m *instrumentorsManager) GetRelevantFuncs() map[string]interface{} {
	funcsMap := make(map[string]interface{})
	for _, i := range m.instrumentors {
		for _, f := range i.FuncNames() {
			funcsMap[f] = nil
		}
	}

	return funcsMap
}

func (m *instrumentorsManager) FilterUnusedInstrumentors(target *process.TargetDetails) {
	existingFuncMap := make(map[string]interface{})
	for _, f := range target.Functions {
		existingFuncMap[f.Name] = nil
	}

	for name, inst := range m.instrumentors {
		funcsFound := 0
		for _, instF := range inst.FuncNames() {
			if _, exists := existingFuncMap[instF]; exists {
				funcsFound++
			}
		}

		if funcsFound != len(inst.FuncNames()) {
			if funcsFound > 0 {
				log.Logger.Error(ErrNotAllFuncsFound, "some of expected functions not found - check instrumented functions", "instrumentation_name", name, "funcs_found", funcsFound, "funcs_expected", len(inst.FuncNames()))
			}
			delete(m.instrumentors, name)
		}
	}
}

func registerInstrumentors(m *instrumentorsManager) error {
	insts := []Instrumentor{
		grpc.New(),
		grpcServer.New(),
		httpServer.New(),
		gorillaMux.New(),
	}

	for _, i := range insts {
		err := m.registerInstrumentor(i)
		if err != nil {
			return err
		}
	}

	return nil
}
