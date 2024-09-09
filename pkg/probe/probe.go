// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Event represents an event from eBPF auto-instrumentation.
type Event struct {
	event *probe.Event
}

// Status represents a Span status.
type Status struct {
	status *probe.Status
}

// ProbedSpan represents a span as returned from an eBPF event.
type ProbedSpan struct {
	span *probe.SpanEvent
}

// GetPackage returns the name of the instrumented package.
func (e *Event) GetPackage() string {
	return e.event.Package
}

// GetSpanKind returns the trace.SpanKind for the event.
func (e *Event) GetSpanKind() trace.SpanKind {
	return e.event.Kind
}

// GetProbedSpans returns the spans for the event.
func (e *Event) GetProbedSpans() []*ProbedSpan {
	spans := make([]*ProbedSpan, len(e.event.SpanEvents))
	for _, probedSpan := range e.event.SpanEvents {
		spans = append(spans, &ProbedSpan{span: probedSpan})
	}
	return spans
}

// GetSpanName returns the name of the span.
func (s *ProbedSpan) GetSpanName() string {
	return s.span.SpanName
}

// GetAttributes returns the list of OpenTelemetry attributes for the span.
func (s *ProbedSpan) GetAttributes() []attribute.KeyValue {
	return s.span.Attributes
}

// GetInstrumentedStartTime returns the start time of the span as instrumented by the probe.
func (s *ProbedSpan) GetInstrumentedStartTime() int64 {
	return s.span.StartTime
}

// GetInstrumentedEndTime returns the end time of the span as instrumented by the probe.
func (s *ProbedSpan) GetInstrumentedEndTime() int64 {
	return s.span.EndTime
}

// GetSpanContext returns the trace.SpanContext for the span.
func (s *ProbedSpan) GetSpanContext() *trace.SpanContext {
	return s.span.SpanContext
}

// GetParentSpanContext returns the trace.SpanContext for the parent span (if any).
func (s *ProbedSpan) GetParentSpanContext() *trace.SpanContext {
	return s.span.ParentSpanContext
}

// GetStatus returns the Status of the span.
func (s *ProbedSpan) GetStatus() Status {
	return Status{status: &s.span.Status}
}

// GetTracerName returns the name of the tracer associated with this span.
func (s *ProbedSpan) GetTracerName() string {
	return s.span.TracerName
}

// GetTracerVersion returns the version of the tracer associated with this span.
func (s *ProbedSpan) GetTracerVersion() string {
	return s.span.TracerVersion
}

// GetTracerSchema returns the schema for the tracer associated with this span.
func (s *ProbedSpan) GetTracerSchema() string {
	return s.span.TracerSchema
}

// GetCode returns the OpenTelemetry status code for a span.
func (s Status) GetCode() codes.Code {
	return s.status.Code
}

// GetDescription returns the string for this status.
func (s Status) GetDescription() string {
	return s.status.Description
}
