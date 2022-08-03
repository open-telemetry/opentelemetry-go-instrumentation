package context

import (
	"github.com/cilium/ebpf/link"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/inject"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
)

type InstrumentorContext struct {
	TargetDetails *process.TargetDetails
	Executable    *link.Executable
	Injector      *inject.Injector
}
