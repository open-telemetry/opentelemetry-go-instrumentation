// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsdk provides an implementation of [export.Handler] that uses the
// default OpenTelemetry Go SDK to process and export the telemetry generated
// by auto-instrumentation.
package otelsdk

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/export"
)

// Handler handles telemetry produced by auto-instrumentation by processing
// that telemetry with the default OpenTelemetry Go SDK.
type Handler struct {
	logger         *slog.Logger
	tracerProvider *sdk.TracerProvider

	stopped atomic.Bool
}

var _ export.Handler = (*Handler)(nil)

// New returns a new configured Handler.
func New(ctx context.Context, options ...Option) (*Handler, error) {
	c, err := newConfig(ctx, options)
	if err != nil {
		return nil, err
	}

	tp, err := c.TracerProvider(ctx)
	if err != nil {
		return nil, err
	}

	return &Handler{
		logger:         c.Logger(),
		tracerProvider: tp,
	}, nil
}

// Handles the passed telemetry using the default OpenTelemetry Go SDK.
func (h *Handler) Handle(telemetry *export.Telemetry) error {
	if telemetry == nil || h.stopped.Load() {
		return nil
	}

	if telemetry.HasMetrics() {
		h.logger.Error("unsupported: dropping metric data")
	}

	if telemetry.HasLogs() {
		h.logger.Error("unsupported: dropping log data")
	}

	h.handleTrace(telemetry.Scope(), telemetry.SchemaURL(), telemetry.Spans())

	return nil
}

func (h *Handler) handleTrace(scope pcommon.InstrumentationScope, url string, spans ptrace.SpanSlice) {
	var (
		startOpts []trace.SpanStartOption
		eventOpts []trace.EventOption
		endOpts   []trace.SpanEndOption
		spanKVs   []attribute.KeyValue
	)

	tracer := h.tracerProvider.Tracer(
		scope.Name(),
		trace.WithInstrumentationVersion(scope.Version()),
		trace.WithInstrumentationAttributes(attrs(scope.Attributes())...),
		trace.WithSchemaURL(url),
	)

	for k := range spans.Len() {
		pSpan := spans.At(k)

		if pSpan.TraceID().IsEmpty() || pSpan.SpanID().IsEmpty() {
			h.logger.Debug("dropping invalid span", "name", pSpan.Name())
			continue
		}
		h.logger.Debug("handling span", "span", pSpan)

		ctx := context.Background()
		if !pSpan.ParentSpanID().IsEmpty() {
			psc := trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: trace.TraceID(pSpan.TraceID()),
				SpanID:  trace.SpanID(pSpan.ParentSpanID()),
			})
			ctx = trace.ContextWithSpanContext(ctx, psc)
		}
		ctx = contextWithSpan(ctx, pSpan)

		spanKVs = appendAttrs(spanKVs, pSpan.Attributes())
		startOpts = append(
			startOpts,
			trace.WithAttributes(spanKVs...),
			trace.WithSpanKind(spanKind(pSpan.Kind())),
			trace.WithTimestamp(pSpan.StartTimestamp().AsTime()),
			trace.WithLinks(h.links(pSpan.Links())...),
		)

		_, span := tracer.Start(ctx, pSpan.Name(), startOpts...)
		startOpts = startOpts[:0]
		spanKVs = spanKVs[:0]

		for l := range pSpan.Events().Len() {
			e := pSpan.Events().At(l)
			eventOpts = appendEventOpts(eventOpts, e)
			span.AddEvent(e.Name(), eventOpts...)
			eventOpts = eventOpts[:0]
		}

		c, msg := status(pSpan.Status())
		span.SetStatus(c, msg)

		endOpts = append(endOpts, trace.WithTimestamp(pSpan.EndTimestamp().AsTime()))
		span.End(endOpts...)
		endOpts = endOpts[:0]
	}
}

// Shutdown shuts down the Handler.
//
// Once shut down, calls to Handle will be dropped.
func (h *Handler) Shutdown(ctx context.Context) error {
	if h.stopped.Swap(true) {
		return nil
	}

	return h.tracerProvider.Shutdown(ctx)
}

func attrs(m pcommon.Map) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, m.Len())
	out = appendAttrs(out, m)
	return out
}

func appendAttrs(dest []attribute.KeyValue, m pcommon.Map) []attribute.KeyValue {
	m.Range(func(k string, v pcommon.Value) bool {
		dest = append(dest, attr(k, v))
		return true
	})
	return dest
}

func attr(k string, v pcommon.Value) attribute.KeyValue {
	return attribute.KeyValue{
		Key:   attribute.Key(k),
		Value: val(v),
	}
}

func val(val pcommon.Value) (out attribute.Value) {
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
			for i := range s.Len() {
				v[i] = s.At(i).Bool()
			}
			out = attribute.BoolSliceValue(v)
		case pcommon.ValueTypeStr:
			v := make([]string, s.Len())
			for i := range s.Len() {
				v[i] = s.At(i).Str()
			}
			out = attribute.StringSliceValue(v)
		case pcommon.ValueTypeInt:
			v := make([]int64, s.Len())
			for i := range s.Len() {
				v[i] = s.At(i).Int()
			}
			out = attribute.Int64SliceValue(v)
		case pcommon.ValueTypeDouble:
			v := make([]float64, s.Len())
			for i := range s.Len() {
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

func spanKind(kind ptrace.SpanKind) trace.SpanKind {
	switch kind {
	case ptrace.SpanKindInternal:
		return trace.SpanKindInternal
	case ptrace.SpanKindServer:
		return trace.SpanKindServer
	case ptrace.SpanKindClient:
		return trace.SpanKindClient
	case ptrace.SpanKindProducer:
		return trace.SpanKindProducer
	case ptrace.SpanKindConsumer:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindUnspecified
	}
}

func appendEventOpts(dest []trace.EventOption, e ptrace.SpanEvent) []trace.EventOption {
	ts := e.Timestamp().AsTime()
	if !ts.IsZero() {
		dest = append(dest, trace.WithTimestamp(ts))
	}

	kvs := attrs(e.Attributes())
	if len(kvs) > 0 {
		dest = append(dest, trace.WithAttributes(kvs...))
	}
	return dest
}

func (h *Handler) links(links ptrace.SpanLinkSlice) []trace.Link {
	n := links.Len()
	if n == 0 {
		return nil
	}

	out := make([]trace.Link, n)
	for i := range out {
		l := links.At(i)

		raw := l.TraceState().AsRaw()
		ts, err := trace.ParseTraceState(raw)
		if err != nil {
			h.logger.Error("failed to parse link tracestate", "error", err, "tracestate", raw)
		}

		out[i] = trace.Link{
			SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    trace.TraceID(l.TraceID()),
				SpanID:     trace.SpanID(l.SpanID()),
				TraceFlags: trace.TraceFlags(l.Flags()),
				TraceState: ts,
			}),
			Attributes: attrs(l.Attributes()),
		}
	}
	return out
}

func status(stat ptrace.Status) (codes.Code, string) {
	var c codes.Code
	switch stat.Code() {
	case ptrace.StatusCodeUnset:
		c = codes.Unset
	case ptrace.StatusCodeOk:
		c = codes.Ok
	case ptrace.StatusCodeError:
		c = codes.Error
	}
	return c, stat.Message()
}
