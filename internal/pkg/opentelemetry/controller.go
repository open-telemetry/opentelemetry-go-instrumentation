// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opentelemetry provides an export and processing pipeline for
// auto-instrumentation telemetry using the OpenTelemetry default SDK.
package opentelemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Controller handles OpenTelemetry telemetry generation for events.
type Controller struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
}

// Trace creates a trace span for event.
//
// This method is safe to call concurrently.
func (c *Controller) Trace(ss ptrace.ScopeSpans) {
	var (
		startOpts []trace.SpanStartOption
		eventOpts []trace.EventOption
		endOpts   []trace.SpanEndOption
		kvs       []attribute.KeyValue
	)

	to := []trace.TracerOption{
		trace.WithInstrumentationVersion(ss.Scope().Version()),
		trace.WithSchemaURL(ss.SchemaUrl()),
	}

	if m := ss.Scope().Attributes(); m.Len() > 0 {
		to = append(to, trace.WithInstrumentationAttributes(attrs(m)...))
	}

	tracer := c.tracerProvider.Tracer(ss.Scope().Name(), to...)
	for k := 0; k < ss.Spans().Len(); k++ {
		pSpan := ss.Spans().At(k)

		if pSpan.TraceID().IsEmpty() || pSpan.SpanID().IsEmpty() {
			c.logger.Debug("dropping invalid span", "name", pSpan.Name())
			continue
		}
		c.logger.Debug("handling span", "tracer", tracer, "span", pSpan)

		ctx := context.Background()
		if !pSpan.ParentSpanID().IsEmpty() {
			psc := trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: trace.TraceID(pSpan.TraceID()),
				SpanID:  trace.SpanID(pSpan.ParentSpanID()),
			})
			ctx = trace.ContextWithSpanContext(ctx, psc)
		}
		ctx = ContextWithSpan(ctx, pSpan)

		kvs = appendAttrs(kvs, pSpan.Attributes())
		startOpts = append(
			startOpts,
			trace.WithAttributes(kvs...),
			trace.WithSpanKind(spanKind(pSpan.Kind())),
			trace.WithTimestamp(pSpan.StartTimestamp().AsTime()),
			trace.WithLinks(c.links(pSpan.Links())...),
		)
		_, span := tracer.Start(ctx, pSpan.Name(), startOpts...)
		startOpts = startOpts[:0]
		kvs = kvs[:0]

		for l := 0; l < pSpan.Events().Len(); l++ {
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

// NewController returns a new initialized [Controller].
func NewController(logger *slog.Logger, tracerProvider trace.TracerProvider) (*Controller, error) {
	return &Controller{
		logger:         logger,
		tracerProvider: tracerProvider,
	}, nil
}

// Shutdown shuts down the OpenTelemetry TracerProvider.
//
// Once shut down, calls to Trace will result in no-op spans (i.e. dropped).
func (c *Controller) Shutdown(ctx context.Context) error {
	if s, ok := c.tracerProvider.(interface {
		Shutdown(context.Context) error
	}); ok {
		// Default TracerProvider implementation.
		return s.Shutdown(ctx)
	}
	return nil
}

func attrs(m pcommon.Map) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, m.Len())
	return appendAttrs(out, m)
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

func (c *Controller) links(links ptrace.SpanLinkSlice) []trace.Link {
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
			c.logger.Error("failed to parse link tracestate", "error", err, "tracestate", raw)
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
