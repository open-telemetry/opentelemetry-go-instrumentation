# Design Proposal: Integration with Manual Instrumentation

## Motivation

Users may want to enrich the traces produced by the automatic instrumentation with additional spans created manually via the [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go).

## Overview

Integration with manual instrumentation happens in two steps:

1. **Modify spans created manually** - attach a uprobe to the function that creates the span, override the trace id and parent span id with the current active span (according to the eBPF map described in the context propagation document).
2. **Update active span map** - After the span is created, update the eBPF map with the new span as the current span. This step is needed in order to create traces that combines spans created manually, automatically (via other probes) and remotely (via context propagation).

This implementation depends on changes described in the [Context Propagation design document](context-propagation.md) and can't be implemented before context propagation is implemented.

## Instrumenting OpenTelemetry Go SDK

The following function (located in `tracer.go` file) may be a good candidate for instrumenting the creation of manual spans:

```go
func (tr *tracer) newRecordingSpan(psc, sc trace.SpanContext, name string, sr SamplingResult, config *trace.SpanConfig) *recordingSpan {
```

By overriding `psc.spanID` and `sc.traceID` to match the current span according to the eBPF map, the function will create a span that is a child of the current active span.

## Future Work

### Use single exporter

Applications instrumented both manually and automatically will export the produced spans via two different exporters. One created manually by the user and another one created in the instrumentation agent. This is not damaging the combined traces, but it is not ideal. In the future, we may want to implement an exporter that communicates with the instrumentation agent (via mechanisem like Unix domain socket) and exports the combined traces over a single connection.
