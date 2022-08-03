package instrumentors

import (
	"fmt"
	gorillaMux "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/github.com/gorilla/mux"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc"
	grpcServer "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/google/golang/org/grpc/server"
	httpServer "github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/bpf/net/http/server"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/goroutine"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/opentelemetry"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
)

type instrumentorsManager struct {
	goroutineTracker *goroutine.Tracker
	instrumentors    map[string]Instrumentor
	done             chan bool
	incomingEvents   chan *events.Event
	otelController   *opentelemetry.Controller
}

func NewManager(otelController *opentelemetry.Controller) (*instrumentorsManager, error) {
	m := &instrumentorsManager{
		instrumentors:    make(map[string]Instrumentor),
		done:             make(chan bool, 1),
		incomingEvents:   make(chan *events.Event),
		otelController:   otelController,
		goroutineTracker: goroutine.NewTracker(),
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

	// Add goroutine tracker functions
	for _, f := range m.goroutineTracker.FuncNames() {
		funcsMap[f] = nil
	}

	return funcsMap
}

func (m *instrumentorsManager) FilterUnusedInstrumentors(target *process.TargetDetails) {
	existingFuncMap := make(map[string]interface{})
	for _, f := range target.Functions {
		existingFuncMap[f.Name] = nil
	}

	for name, inst := range m.instrumentors {
		someFuncExists := false
		for _, instF := range inst.FuncNames() {
			if _, exists := existingFuncMap[instF]; exists {
				someFuncExists = true
				break
			}
		}

		if !someFuncExists {
			log.Logger.V(1).Info("filtering unused instrumentation", "name", name)
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
