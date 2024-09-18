// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"log/slog"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// maxSize is the maximum payload size of binary data transported in an event.
// This needs to remain in sync with the eBPF program.
const maxSize = 1024

// New returns a new [probe.Probe].
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: "go.opentelemetry.io/auto",
	}
	c := &converter{logger: logger}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "span_context_trace_id_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"traceID",
				),
			},
			probe.StructFieldConst{
				Key: "span_context_span_id_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"spanID",
				),
			},
			probe.StructFieldConst{
				Key: "span_context_trace_flags_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"traceFlags",
				),
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:        "go.opentelemetry.io/auto/sdk.(*tracer).start",
				EntryProbe: "uprobe_Tracer_start",
			},
			{
				Sym:        "go.opentelemetry.io/auto/sdk.(*span).ended",
				EntryProbe: "uprobe_Span_ended",
			},
		},
		SpecFn:    loadBpf,
		ProcessFn: c.convertEvent,
	}
}

type event struct {
	Size     uint32
	SpanData [maxSize]byte
}

type converter struct {
	logger *slog.Logger
}

func (c *converter) convertEvent(e *event) []*probe.SpanEvent {
	var m ptrace.ProtoUnmarshaler
	traces, err := m.UnmarshalTraces(e.SpanData[:e.Size])
	if err != nil {
		c.logger.Error("failed to unmarshal span data", "error", err)
		return nil
	}

	ss := traces.ResourceSpans().At(0).ScopeSpans().At(0) // TODO: validate len before lookup.
	span := ss.Spans().At(0)                              // TODO: validate len before lookup.

	raw := span.TraceState().AsRaw()
	ts, err := trace.ParseTraceState(raw)
	if err != nil {
		c.logger.Error("failed to parse tracestate", "error", err, "tracestate", raw)
	}

	var pscPtr *trace.SpanContext
	if psid := span.ParentSpanID(); psid != pcommon.NewSpanIDEmpty() {
		psc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID(span.TraceID()),
			SpanID:     trace.SpanID(psid),
			TraceFlags: trace.TraceFlags(span.Flags()),
			TraceState: ts,
		})
		pscPtr = &psc
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(span.TraceID()),
		SpanID:     trace.SpanID(span.SpanID()),
		TraceFlags: trace.TraceFlags(span.Flags()),
		TraceState: ts,
	})
	span.ParentSpanID()
	return []*probe.SpanEvent{{
		SpanName:          span.Name(),
		StartTime:         span.StartTimestamp().AsTime(),
		EndTime:           span.EndTimestamp().AsTime(),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		TracerName:        ss.Scope().Name(),
		TracerVersion:     ss.Scope().Version(),
		TracerSchema:      ss.SchemaUrl(),
		// TODO: Status.
		// TODO: Attributes.
		// TODO: Events.
		// TODO: Links.
		// TODO: Span Kind.
	}}
}
