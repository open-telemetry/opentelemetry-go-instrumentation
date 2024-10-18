// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"
)

type EBPFSourceIDGenerator struct{}

type eBPFEventKey struct{}

func NewEBPFSourceIDGenerator() *EBPFSourceIDGenerator {
	return &EBPFSourceIDGenerator{}
}

// ContextWithSpan returns a copy of parent in which span is stored.
func ContextWithSpan(parent context.Context, span ptrace.Span) context.Context {
	return context.WithValue(parent, eBPFEventKey{}, span)
}

// SpanFromContext returns the Span within ctx if one exists.
func SpanFromContext(ctx context.Context) ptrace.Span {
	val := ctx.Value(eBPFEventKey{})
	if val == nil {
		return ptrace.NewSpan()
	}

	s, _ := val.(ptrace.Span)
	return s
}

func (e *EBPFSourceIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	s := SpanFromContext(ctx)
	if s.TraceID().IsEmpty() || s.SpanID().IsEmpty() {
		return trace.TraceID{}, trace.SpanID{}
	}

	return trace.TraceID(s.TraceID()), trace.SpanID(s.SpanID())
}

func (e *EBPFSourceIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	return trace.SpanID(SpanFromContext(ctx).SpanID())
}
