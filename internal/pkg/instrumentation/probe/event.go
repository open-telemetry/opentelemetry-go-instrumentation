// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Event is a telemetry event that happens within an instrumented package.
type Event struct {
	// Package is the name of the instrumented package.
	Package    string
	// Kind is the trace.SpanKind for the event.
	Kind       trace.SpanKind
	// SpanEvents is a list of spans for this event.
	SpanEvents []*SpanEvent
}

// Status represents the OpenTelemetry status code and description for a span.
type Status struct {
	// Code is the OpenTelemetry status code for a span.
	Code        codes.Code
	// Description is the string for this status.
	Description string
}

// SpanEvent represents a probed span.
type SpanEvent struct {
	// SpanName is the name of the span.
	SpanName          string
	// Attributes is a list of OpenTelemetry attributes for the span.
	Attributes        []attribute.KeyValue
	// StartTime is the start time for the span.
	StartTime         int64
	// EndTime is the end time for the span.
	EndTime           int64
	// SpanContext is the context for this span.
	SpanContext       *trace.SpanContext
	// ParentSpanContext is the context for this span's parent (if applicable).
	ParentSpanContext *trace.SpanContext
	// Status is the status of this span.
	Status            Status
	// TracerName is the name of the tracer associated with this span.
	TracerName        string
	// TracerVersion is the version of the tracer associated with this span.
	TracerVersion     string
	// TracerSchema is the schema for the tracer associated with this span.
	TracerSchema      string
}
