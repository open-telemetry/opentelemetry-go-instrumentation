package events

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Event struct {
	Library      string
	Name         string
	Attributes   []attribute.KeyValue
	Kind         trace.SpanKind
	GoroutineUID int64
	StartTime    int64
	EndTime      int64
}
