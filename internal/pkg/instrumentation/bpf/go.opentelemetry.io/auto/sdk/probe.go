// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cilium/ebpf/perf"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
	"go.opentelemetry.io/auto/sdk/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

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
		SpecFn:        loadBpf,
		ProcessFn:     c.convertEvent,
		ProcessRecord: c.decodeEvent,
	}
}

type event struct {
	Size     uint32
	SpanData []byte
}

type converter struct {
	logger *slog.Logger
}

func (c *converter) decodeEvent(record perf.Record) (event, error) {
	reader := bytes.NewReader(record.RawSample)

	var e event
	err := binary.Read(reader, binary.LittleEndian, &e.Size)
	if err != nil {
		c.logger.Error("failed to decode size", "error", err)
		return event{}, err
	}
	c.logger.Debug("decoded size", "size", e.Size)

	e.SpanData = make([]byte, e.Size)
	_, err = reader.Read(e.SpanData)
	if err != nil {
		c.logger.Error("failed to read span data", "error", err)
		return event{}, err
	}
	c.logger.Debug("decoded span data", "size", e.Size)
	return e, nil
}

func (c *converter) convertEvent(e *event) []*probe.SpanEvent {
	var traces telemetry.Traces
	err := json.Unmarshal(e.SpanData[:e.Size], &traces)
	if err != nil {
		c.logger.Error("failed to unmarshal span data", "error", err)
		return nil
	}

	switch {
	case len(traces.ResourceSpans) == 0:
		c.logger.Error("empty ResourceSpans")
		return nil
	case len(traces.ResourceSpans[0].ScopeSpans) == 0:
		c.logger.Error("empty ScopeSpans")
		return nil
	case len(traces.ResourceSpans[0].ScopeSpans[0].Spans) == 0:
		c.logger.Error("empty Spans")
		return nil
	}

	ss := traces.ResourceSpans[0].ScopeSpans[0]
	span := ss.Spans[0]

	ts, err := trace.ParseTraceState(span.TraceState)
	if err != nil {
		c.logger.Error("failed to parse tracestate", "error", err, "tracestate", span.TraceState)
	}

	var pscPtr *trace.SpanContext
	if psid := span.ParentSpanID; !psid.IsEmpty() {
		psc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID(span.TraceID),
			SpanID:     trace.SpanID(psid),
			TraceFlags: trace.TraceFlags(span.Flags),
			TraceState: ts,
		})
		pscPtr = &psc
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(span.TraceID),
		SpanID:     trace.SpanID(span.SpanID),
		TraceFlags: trace.TraceFlags(span.Flags),
		TraceState: ts,
	})
	return []*probe.SpanEvent{{
		SpanName:          span.Name,
		StartTime:         span.StartTime,
		EndTime:           span.EndTime,
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		TracerName:        ss.Scope.Name,
		TracerVersion:     ss.Scope.Version,
		TracerSchema:      ss.SchemaURL,
		Kind:              spanKind(span.Kind),
		Attributes:        attributes(span.Attrs),
		Events:            events(span.Events),
		Links:             c.links(span.Links),
		Status:            status(span.Status),
	}}
}

func spanKind(kind telemetry.SpanKind) trace.SpanKind {
	switch kind {
	case telemetry.SpanKindInternal:
		return trace.SpanKindInternal
	case telemetry.SpanKindServer:
		return trace.SpanKindServer
	case telemetry.SpanKindClient:
		return trace.SpanKindClient
	case telemetry.SpanKindProducer:
		return trace.SpanKindProducer
	case telemetry.SpanKindConsumer:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindUnspecified
	}
}

func events(e []*telemetry.SpanEvent) map[string][]trace.EventOption {
	out := make(map[string][]trace.EventOption)
	for _, event := range e {
		var opts []trace.EventOption

		ts := event.Time
		if !ts.IsZero() {
			opts = append(opts, trace.WithTimestamp(ts))
		}

		attrs := attributes(event.Attrs)
		if len(attrs) > 0 {
			opts = append(opts, trace.WithAttributes(attrs...))
		}

		out[event.Name] = opts
	}
	return out
}

func (c *converter) links(links []*telemetry.SpanLink) []trace.Link {
	out := make([]trace.Link, len(links))
	for i, l := range links {
		ts, err := trace.ParseTraceState(l.TraceState)
		if err != nil {
			c.logger.Error("failed to parse link tracestate", "error", err, "tracestate", l.TraceState)
		}

		out[i] = trace.Link{
			SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    trace.TraceID(l.TraceID),
				SpanID:     trace.SpanID(l.SpanID),
				TraceFlags: trace.TraceFlags(l.Flags),
				TraceState: ts,
			}),
			Attributes: attributes(l.Attrs),
		}
	}
	return out
}

func attributes(attrs []telemetry.Attr) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, attribute.KeyValue{
			Key:   attribute.Key(a.Key),
			Value: attributeValue(a.Value),
		})
	}
	return out
}

func attributeValue(val telemetry.Value) (out attribute.Value) {
	switch val.Kind() {
	// ValueKindBytes and ValueKindMap not supported as they are invalid input
	// types from the OTel Go trace API.
	case telemetry.ValueKindEmpty:
	case telemetry.ValueKindString:
		out = attribute.StringValue(val.AsString())
	case telemetry.ValueKindInt64:
		out = attribute.Int64Value(val.AsInt64())
	case telemetry.ValueKindFloat64:
		out = attribute.Float64Value(val.AsFloat64())
	case telemetry.ValueKindBool:
		out = attribute.BoolValue(val.AsBool())
	case telemetry.ValueKindSlice:
		s := val.AsSlice()
		if len(s) == 0 {
			// Undetectable slice type.
			out = attribute.StringValue("<empty slice>")
			return out
		}

		// Validate homogeneity before allocating.
		var k telemetry.ValueKind
		for i, v := range s {
			if i == 0 {
				k = v.Kind()
			} else {
				if v.Kind() != k {
					out = attribute.StringValue("<inhomogeneous slice>")
					return out
				}
			}
		}

		switch k {
		case telemetry.ValueKindBool:
			v := make([]bool, len(s))
			for i := 0; i < len(s); i++ {
				v[i] = s[i].AsBool()
			}
			out = attribute.BoolSliceValue(v)
		case telemetry.ValueKindString:
			v := make([]string, len(s))
			for i := 0; i < len(s); i++ {
				v[i] = s[i].AsString()
			}
			out = attribute.StringSliceValue(v)
		case telemetry.ValueKindInt64:
			v := make([]int64, len(s))
			for i := 0; i < len(s); i++ {
				v[i] = s[i].AsInt64()
			}
			out = attribute.Int64SliceValue(v)
		case telemetry.ValueKindFloat64:
			v := make([]float64, len(s))
			for i := 0; i < len(s); i++ {
				v[i] = s[i].AsFloat64()
			}
			out = attribute.Float64SliceValue(v)
		default:
			out = attribute.StringValue(fmt.Sprintf("<invalid slice type %s>", k.String()))
		}
	default:
		out = attribute.StringValue(fmt.Sprintf("<unknown: %s>", val.String()))
	}
	return out
}

func status(stat *telemetry.Status) probe.Status {
	if stat == nil {
		return probe.Status{}
	}

	var c codes.Code
	switch stat.Code {
	case telemetry.StatusCodeUnset:
		c = codes.Unset
	case telemetry.StatusCodeOK:
		c = codes.Ok
	case telemetry.StatusCodeError:
		c = codes.Error
	}
	return probe.Status{
		Code:        c,
		Description: stat.Message,
	}
}
