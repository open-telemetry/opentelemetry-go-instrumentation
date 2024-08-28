// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"fmt"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/go-logr/logr"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: "go.opentelemetry.io/auto",
	}
	logger = logger.WithName(id.String())
	c := &converter{logger: logger.WithName("converter")}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:        "go.opentelemetry.io/auto/internal/sdk.(*Span).ended",
				EntryProbe: "uprobe_Span_ended",
			},
		},
		SpecFn:    loadBpf,
		ProcessFn: c.convertEvent,
	}
}

type event struct {
	Size     uint32
	SpanData [256]byte
}

type converter struct {
	logger logr.Logger
}

func (c *converter) convertEvent(e *event) []*probe.SpanEvent {
	var m ptrace.ProtoUnmarshaler
	traces, err := m.UnmarshalTraces(e.SpanData[:e.Size]) // TODO: handle returned error.
	if err != nil {
		c.logger.Error(err, "failed to unmarshal span data")
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			SpanID: trace.SpanID{1},
		})
		return []*probe.SpanEvent{{
			SpanName:    "error",
			SpanContext: &sc,
		}}
	}

	ss := traces.ResourceSpans().At(0).ScopeSpans().At(0) // TODO: validate len before lookup.
	span := ss.Spans().At(0)                              // TODO: validate len before lookup.

	raw := span.TraceState().AsRaw()
	ts, err := trace.ParseTraceState(raw)
	if err != nil {
		c.logger.Error(err, "failed to parse tracestate", "tracestate", raw)
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
		Attributes:        attributes(span.Attributes()),
		StartTime:         span.StartTimestamp().AsTime().UnixNano(),
		EndTime:           span.EndTimestamp().AsTime().UnixNano(),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		Status:            status(span.Status()),
		TracerName:        ss.Scope().Name(),
		TracerVersion:     ss.Scope().Version(),
		TracerSchema:      ss.SchemaUrl(),
		// TODO: span events.
		// TODO: span links.
		// TODO: span kind.
	}}
}

func attributes(m pcommon.Map) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, m.Len())
	m.Range(func(key string, val pcommon.Value) bool {
		out = append(out, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: attributeValue(val),
		})
		return true
	})
	return out
}

func attributeValue(val pcommon.Value) (out attribute.Value) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
	case pcommon.ValueTypeStr:
		out = attribute.StringValue(val.AsString())
	case pcommon.ValueTypeInt:
		out = attribute.Int64Value(val.Int())
	case pcommon.ValueTypeDouble:
		out = attribute.Float64Value(val.Double())
	case pcommon.ValueTypeBool:
		out = attribute.BoolValue(val.Bool())
	case pcommon.ValueTypeSlice:
		s := val.Slice()
		if s.Len() == 0 {
			// Undetectable slice type.
			out = attribute.StringValue("<empty slice>")
			return out
		}

		// Validate homogeneity before allocating.
		t := s.At(0).Type()
		for i := 1; i < s.Len(); i++ {
			if s.At(i).Type() != t {
				out = attribute.StringValue("<inhomogeneous slice>")
				return out
			}
		}

		switch t {
		case pcommon.ValueTypeBool:
			v := make([]bool, s.Len())
			for i := 0; i < s.Len(); i++ {
				v[i] = s.At(i).Bool()
			}
			out = attribute.BoolSliceValue(v)
		case pcommon.ValueTypeStr:
			v := make([]string, s.Len())
			for i := 0; i < s.Len(); i++ {
				v[i] = s.At(i).Str()
			}
			out = attribute.StringSliceValue(v)
		case pcommon.ValueTypeInt:
			v := make([]int64, s.Len())
			for i := 0; i < s.Len(); i++ {
				v[i] = s.At(i).Int()
			}
			out = attribute.Int64SliceValue(v)
		case pcommon.ValueTypeDouble:
			v := make([]float64, s.Len())
			for i := 0; i < s.Len(); i++ {
				v[i] = s.At(i).Double()
			}
			out = attribute.Float64SliceValue(v)
		default:
			out = attribute.StringValue(fmt.Sprintf("<invalid slice type %s>", t.String()))
		}
	default:
		out = attribute.StringValue(fmt.Sprintf("<unknown: %#v>", val.AsRaw()))
	}
	return out
}

func status(stat ptrace.Status) probe.Status {
	return probe.Status{
		Code:        codes.Code(stat.Code()),
		Description: stat.Message(),
	}
}
