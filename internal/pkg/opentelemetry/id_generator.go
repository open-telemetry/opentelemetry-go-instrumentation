// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

type EBPFSourceIDGenerator struct{}

type eBPFEventKey struct{}

func NewEBPFSourceIDGenerator() *EBPFSourceIDGenerator {
	return &EBPFSourceIDGenerator{}
}

// ContextWithEBPFEvent returns a copy of parent in which event is stored.
func ContextWithEBPFEvent(parent context.Context, event probe.SpanEvent) context.Context {
	return context.WithValue(parent, eBPFEventKey{}, event)
}

// EventFromContext returns the event within ctx if one exists.
func EventFromContext(ctx context.Context) *probe.SpanEvent {
	val := ctx.Value(eBPFEventKey{})
	if val == nil {
		return nil
	}

	event, ok := val.(probe.SpanEvent)
	if !ok {
		return nil
	}

	return &event
}

func (e *EBPFSourceIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	event := EventFromContext(ctx)
	if event == nil || event.SpanContext == nil {
		return trace.TraceID{}, trace.SpanID{}
	}

	return event.SpanContext.TraceID(), event.SpanContext.SpanID()
}

func (e *EBPFSourceIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	event := EventFromContext(ctx)
	if event == nil {
		return trace.SpanID{}
	}

	return event.SpanContext.SpanID()
}
