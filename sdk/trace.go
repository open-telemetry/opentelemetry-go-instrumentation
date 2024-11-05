// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.opentelemetry.io/auto/sdk/internal/telemetry"
)

// TracerProvider returns an auto-instrumentable [trace.TracerProvider].
//
// If an [go.opentelemetry.io/auto.Instrumentation] is configured to instrument
// the process using the returned TracerProvider, all of the telemetry it
// produces will be processed and handled by that Instrumentation. By default,
// if no Instrumentation instruments the TracerProvider it will not generate
// any trace telemetry.
func TracerProvider() trace.TracerProvider { return tracerProviderInstance }

var tracerProviderInstance = tracerProvider{}

type tracerProvider struct{ noop.TracerProvider }

var _ trace.TracerProvider = tracerProvider{}

func (p tracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	cfg := trace.NewTracerConfig(opts...)
	return tracer{
		name:      name,
		version:   cfg.InstrumentationVersion(),
		schemaURL: cfg.SchemaURL(),
	}
}

type tracer struct {
	noop.Tracer

	name, schemaURL, version string
}

var _ trace.Tracer = tracer{}

func (t tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	var psc trace.SpanContext
	span := &span{sampled: true}

	// Ask eBPF for sampling decision and span context info.
	t.start(ctx, span, &psc, &span.sampled, &span.spanContext)

	ctx = trace.ContextWithSpan(ctx, span)

	if span.sampled {
		// Only build traces if sampled.
		cfg := trace.NewSpanStartConfig(opts...)
		span.traces, span.span = t.traces(ctx, name, cfg, span.spanContext, psc)
	}

	return ctx, span
}

// Expected to be implemented in eBPF.
//
//go:noinline
func (t *tracer) start(
	ctx context.Context,
	spanPtr *span,
	psc *trace.SpanContext,
	sampled *bool,
	sc *trace.SpanContext,
) {
	start(ctx, spanPtr, psc, sampled, sc)
}

// start is used for testing.
var start = func(context.Context, *span, *trace.SpanContext, *bool, *trace.SpanContext) {}

func (t tracer) traces(ctx context.Context, name string, cfg trace.SpanConfig, sc, psc trace.SpanContext) (*telemetry.Traces, *telemetry.Span) {
	span := &telemetry.Span{
		TraceID:      telemetry.TraceID(sc.TraceID()),
		SpanID:       telemetry.SpanID(sc.SpanID()),
		Flags:        uint32(sc.TraceFlags()),
		TraceState:   sc.TraceState().String(),
		ParentSpanID: telemetry.SpanID(psc.SpanID()),
		Name:         name,
		Kind:         spanKind(cfg.SpanKind()),
		Attrs:        convAttrs(cfg.Attributes()),
		Links:        convLinks(cfg.Links()),
	}

	if t := cfg.Timestamp(); !t.IsZero() {
		span.StartTime = cfg.Timestamp()
	} else {
		span.StartTime = time.Now()
	}

	return &telemetry.Traces{
		ResourceSpans: []*telemetry.ResourceSpans{
			{
				ScopeSpans: []*telemetry.ScopeSpans{
					{
						Scope: &telemetry.Scope{
							Name:    t.name,
							Version: t.version,
						},
						Spans:     []*telemetry.Span{span},
						SchemaURL: t.schemaURL,
					},
				},
			},
		},
	}, span
}

func spanKind(kind trace.SpanKind) telemetry.SpanKind {
	switch kind {
	case trace.SpanKindInternal:
		return telemetry.SpanKindInternal
	case trace.SpanKindServer:
		return telemetry.SpanKindServer
	case trace.SpanKindClient:
		return telemetry.SpanKindClient
	case trace.SpanKindProducer:
		return telemetry.SpanKindProducer
	case trace.SpanKindConsumer:
		return telemetry.SpanKindConsumer
	}
	return telemetry.SpanKind(0) // undefined.
}

type span struct {
	noop.Span

	sampled     bool
	spanContext trace.SpanContext

	traces *telemetry.Traces
	span   *telemetry.Span
}

func (s *span) SpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}
	return s.spanContext
}

func (s *span) IsRecording() bool {
	if s == nil {
		return false
	}
	return s.sampled
}

func (s *span) SetStatus(c codes.Code, msg string) {
	if s == nil || !s.sampled {
		return
	}

	if s.span.Status == nil {
		s.span.Status = new(telemetry.Status)
	}

	s.span.Status.Message = msg

	switch c {
	case codes.Unset:
		s.span.Status.Code = telemetry.StatusCodeUnset
	case codes.Error:
		s.span.Status.Code = telemetry.StatusCodeError
	case codes.Ok:
		s.span.Status.Code = telemetry.StatusCodeOK
	}
}

func (s *span) SetAttributes(attrs ...attribute.KeyValue) {
	if s == nil || !s.sampled {
		return
	}

	// TODO: handle attribute limits.

	m := make(map[string]int)
	for i, a := range s.span.Attrs {
		m[a.Key] = i
	}

	for _, a := range attrs {
		val := convAttrValue(a.Value)
		if val.Empty() {
			continue
		}

		if idx, ok := m[string(a.Key)]; ok {
			s.span.Attrs[idx] = telemetry.Attr{
				Key:   string(a.Key),
				Value: val,
			}
		} else {
			s.span.Attrs = append(s.span.Attrs, telemetry.Attr{
				Key:   string(a.Key),
				Value: val,
			})
			m[string(a.Key)] = len(s.span.Attrs) - 1
		}
	}
}

func convAttrs(attrs []attribute.KeyValue) []telemetry.Attr {
	out := make([]telemetry.Attr, 0, len(attrs))
	for _, attr := range attrs {
		key := string(attr.Key)
		val := convAttrValue(attr.Value)
		if val.Empty() {
			continue
		}
		out = append(out, telemetry.Attr{Key: key, Value: val})
	}
	return out
}

func convAttrValue(value attribute.Value) telemetry.Value {
	switch value.Type() {
	case attribute.BOOL:
		return telemetry.BoolValue(value.AsBool())
	case attribute.INT64:
		return telemetry.Int64Value(value.AsInt64())
	case attribute.FLOAT64:
		return telemetry.Float64Value(value.AsFloat64())
	case attribute.STRING:
		return telemetry.StringValue(value.AsString())
	case attribute.BOOLSLICE:
		slice := value.AsBoolSlice()
		out := make([]telemetry.Value, 0, len(slice))
		for _, v := range slice {
			out = append(out, telemetry.BoolValue(v))
		}
		return telemetry.SliceValue(out...)
	case attribute.INT64SLICE:
		slice := value.AsInt64Slice()
		out := make([]telemetry.Value, 0, len(slice))
		for _, v := range slice {
			out = append(out, telemetry.Int64Value(v))
		}
		return telemetry.SliceValue(out...)
	case attribute.FLOAT64SLICE:
		slice := value.AsFloat64Slice()
		out := make([]telemetry.Value, 0, len(slice))
		for _, v := range slice {
			out = append(out, telemetry.Float64Value(v))
		}
		return telemetry.SliceValue(out...)
	case attribute.STRINGSLICE:
		slice := value.AsStringSlice()
		out := make([]telemetry.Value, 0, len(slice))
		for _, v := range slice {
			out = append(out, telemetry.StringValue(v))
		}
		return telemetry.SliceValue(out...)
	}
	return telemetry.Value{}
}

func (s *span) End(opts ...trace.SpanEndOption) {
	if s == nil || !s.sampled {
		return
	}

	cfg := trace.NewSpanEndConfig(opts...)
	if t := cfg.Timestamp(); !t.IsZero() {
		s.span.EndTime = cfg.Timestamp()
	} else {
		s.span.EndTime = time.Now()
	}

	b, _ := json.Marshal(s.traces) // TODO: do not ignore this error.

	s.sampled = false

	s.ended(b)
}

// Expected to be implemented in eBPF.
//
//go:noinline
func (*span) ended(buf []byte) { ended(buf) }

// ended is used for testing.
var ended = func([]byte) {}

func (s *span) RecordError(err error, opts ...trace.EventOption) {
	if s == nil || err == nil || !s.sampled {
		return
	}

	cfg := trace.NewEventConfig(opts...)

	attrs := cfg.Attributes()
	attrs = append(attrs,
		semconv.ExceptionType(typeStr(err)),
		semconv.ExceptionMessage(err.Error()),
	)
	if cfg.StackTrace() {
		buf := make([]byte, 2048)
		n := runtime.Stack(buf, false)
		attrs = append(attrs, semconv.ExceptionStacktrace(string(buf[0:n])))
	}

	s.addEvent(semconv.ExceptionEventName, cfg.Timestamp(), attrs)
}

func typeStr(i any) string {
	t := reflect.TypeOf(i)
	if t.PkgPath() == "" && t.Name() == "" {
		// Likely a builtin type.
		return t.String()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

func (s *span) AddEvent(name string, opts ...trace.EventOption) {
	if s == nil || !s.sampled {
		return
	}

	cfg := trace.NewEventConfig(opts...)
	s.addEvent(name, cfg.Timestamp(), cfg.Attributes())
}

func (s *span) addEvent(name string, tStamp time.Time, attrs []attribute.KeyValue) {
	// TODO: handle event limits.

	s.span.Events = append(s.span.Events, &telemetry.SpanEvent{
		Time:  tStamp,
		Name:  name,
		Attrs: convAttrs(attrs),
	})
}

func (s *span) AddLink(link trace.Link) {
	if s == nil || !s.sampled {
		return
	}

	// TODO: handle link limits.

	s.span.Links = append(s.span.Links, convLink(link))
}

func convLinks(links []trace.Link) []*telemetry.SpanLink {
	out := make([]*telemetry.SpanLink, 0, len(links))
	for _, link := range links {
		out = append(out, convLink(link))
	}
	return out
}

func convLink(link trace.Link) *telemetry.SpanLink {
	return &telemetry.SpanLink{
		TraceID:    telemetry.TraceID(link.SpanContext.TraceID()),
		SpanID:     telemetry.SpanID(link.SpanContext.SpanID()),
		TraceState: link.SpanContext.TraceState().String(),
		Attrs:      convAttrs(link.Attributes),
		Flags:      uint32(link.SpanContext.TraceFlags()),
	}
}

func (s *span) SetName(name string) {
	if s == nil || !s.sampled {
		return
	}
	s.span.Name = name
}

func (*span) TracerProvider() trace.TracerProvider { return TracerProvider() }
