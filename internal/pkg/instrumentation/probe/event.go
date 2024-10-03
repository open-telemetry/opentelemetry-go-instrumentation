// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"time"

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
	Kind              trace.SpanKind
	Attributes        []attribute.KeyValue
	StartTime         time.Time
	EndTime           time.Time
	SpanContext       *trace.SpanContext
	ParentSpanContext *trace.SpanContext
	Status            Status
	TracerName        string
	TracerVersion     string
	TracerSchema      string
	Events            map[string][]trace.EventOption
	Links             []trace.Link
}
