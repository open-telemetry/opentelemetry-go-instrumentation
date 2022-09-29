package opentelemetry

import (
	"context"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"go.opentelemetry.io/otel/trace"
)

type eBPFSourceIDGenerator struct{}

type ebpfEventKey struct{}

func NewEbpfSourceIDGenerator() *eBPFSourceIDGenerator {
	return &eBPFSourceIDGenerator{}
}

func ContextWithEbpfEvent(ctx context.Context, event events.Event) context.Context {
	return context.WithValue(ctx, ebpfEventKey{}, event)
}

func EventFromContext(ctx context.Context) *events.Event {
	val := ctx.Value(ebpfEventKey{})
	if val == nil {
		return nil
	}

	event, ok := val.(events.Event)
	if !ok {
		return nil
	}

	return &event
}

func (e *eBPFSourceIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	event := EventFromContext(ctx)
	if event == nil || event.SpanContext == nil {
		return trace.TraceID{}, trace.SpanID{}
	}
	
	return event.SpanContext.TraceID(), event.SpanContext.SpanID()
}

func (e *eBPFSourceIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	event := EventFromContext(ctx)
	if event == nil {
		return trace.SpanID{}
	}

	return event.SpanContext.SpanID()
}
