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
	Package    string
	Kind       trace.SpanKind
	SpanEvents []*SpanEvent
}

type Status struct {
	Code        codes.Code
	Description string
}

type SpanEvent struct {
	SpanName          string
	Attributes        []attribute.KeyValue
	StartTime         int64
	EndTime           int64
	SpanContext       *trace.SpanContext
	ParentSpanContext *trace.SpanContext
	Status            Status
}
