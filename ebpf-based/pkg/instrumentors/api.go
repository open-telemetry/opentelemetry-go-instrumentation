package instrumentors

import (
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
)

type Instrumentor interface {
	LibraryName() string
	FuncNames() []string
	Load(ctx *context.InstrumentorContext) error
	Run(eventsChan chan<- *events.Event)
	Close()
}
