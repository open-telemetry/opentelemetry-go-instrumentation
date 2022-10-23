package context

import "go.opentelemetry.io/otel/trace"

type EbpfSpanContext struct {
	TraceID trace.TraceID
	SpanID  trace.SpanID
}
