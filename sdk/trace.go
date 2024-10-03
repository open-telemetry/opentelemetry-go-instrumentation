// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// GetTracerProvider returns an auto-instrumentable [trace.TracerProvider].
//
// If an [go.opentelemetry.io/auto.Instrumentation] is configured to instrument
// the process using the returned TracerProvider, all of the telemetry it
// produces will be processed and handled by that Instrumentation. By default,
// if no Instrumentation instruments the TracerProvider it will not generate
// any trace telemetry.
func GetTracerProvider() trace.TracerProvider { return tracerProviderInstance }

var tracerProviderInstance = tracerProvider{}

type tracerProvider struct{ embedded.TracerProvider }

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
	embedded.Tracer

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

func (t tracer) traces(ctx context.Context, name string, cfg trace.SpanConfig, sc, psc trace.SpanContext) (ptrace.Traces, ptrace.Span) {
	// TODO: pool this. It can be returned on end.
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)

	rs := traces.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().EnsureCapacity(1)

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName(t.name)
	ss.Scope().SetVersion(t.version)
	ss.SetSchemaUrl(t.schemaURL)
	ss.Spans().EnsureCapacity(1)

	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID(sc.TraceID()))
	span.SetSpanID(pcommon.SpanID(sc.SpanID()))
	span.SetFlags(uint32(sc.TraceFlags()))
	span.TraceState().FromRaw(sc.TraceState().String())
	span.SetParentSpanID(pcommon.SpanID(psc.SpanID()))
	span.SetName(name)
	span.SetKind(spanKind(cfg.SpanKind()))

	var start pcommon.Timestamp
	if t := cfg.Timestamp(); !t.IsZero() {
		start = pcommon.NewTimestampFromTime(cfg.Timestamp())
	} else {
		start = pcommon.NewTimestampFromTime(time.Now())
	}
	span.SetStartTimestamp(start)
	addLinks(span.Links(), cfg.Links()...)
	setAttributes(span.Attributes(), cfg.Attributes())

	return traces, span
}

func spanKind(kind trace.SpanKind) ptrace.SpanKind {
	switch kind {
	case trace.SpanKindInternal:
		return ptrace.SpanKindInternal
	case trace.SpanKindServer:
		return ptrace.SpanKindServer
	case trace.SpanKindClient:
		return ptrace.SpanKindClient
	case trace.SpanKindProducer:
		return ptrace.SpanKindProducer
	case trace.SpanKindConsumer:
		return ptrace.SpanKindConsumer
	}
	return ptrace.SpanKindUnspecified
}

type span struct {
	embedded.Span

	sampled     bool
	spanContext trace.SpanContext

	traces ptrace.Traces
	span   ptrace.Span
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

	stat := s.span.Status()
	stat.SetMessage(msg)

	switch c {
	case codes.Unset:
		stat.SetCode(ptrace.StatusCodeUnset)
	case codes.Error:
		stat.SetCode(ptrace.StatusCodeError)
	case codes.Ok:
		stat.SetCode(ptrace.StatusCodeOk)
	}
}

func (s *span) SetAttributes(attrs ...attribute.KeyValue) {
	if s == nil || !s.sampled {
		return
	}

	// TODO: handle attribute limits.

	setAttributes(s.span.Attributes(), attrs)
}

func setAttributes(dest pcommon.Map, attrs []attribute.KeyValue) {
	dest.EnsureCapacity(len(attrs))
	for _, attr := range attrs {
		key := string(attr.Key)
		switch attr.Value.Type() {
		case attribute.BOOL:
			dest.PutBool(key, attr.Value.AsBool())
		case attribute.INT64:
			dest.PutInt(key, attr.Value.AsInt64())
		case attribute.FLOAT64:
			dest.PutDouble(key, attr.Value.AsFloat64())
		case attribute.STRING:
			dest.PutStr(key, attr.Value.AsString())
		case attribute.BOOLSLICE:
			val := attr.Value.AsBoolSlice()
			s := dest.PutEmptySlice(key)
			s.EnsureCapacity(len(val))
			for _, v := range val {
				s.AppendEmpty().SetBool(v)
			}
		case attribute.INT64SLICE:
			val := attr.Value.AsInt64Slice()
			s := dest.PutEmptySlice(key)
			s.EnsureCapacity(len(val))
			for _, v := range val {
				s.AppendEmpty().SetInt(v)
			}
		case attribute.FLOAT64SLICE:
			val := attr.Value.AsFloat64Slice()
			s := dest.PutEmptySlice(key)
			s.EnsureCapacity(len(val))
			for _, v := range val {
				s.AppendEmpty().SetDouble(v)
			}
		case attribute.STRINGSLICE:
			val := attr.Value.AsStringSlice()
			s := dest.PutEmptySlice(key)
			s.EnsureCapacity(len(val))
			for _, v := range val {
				s.AppendEmpty().SetStr(v)
			}
		}
	}
}

func (s *span) End(opts ...trace.SpanEndOption) {
	if s == nil || !s.sampled {
		return
	}

	cfg := trace.NewSpanEndConfig(opts...)
	var end time.Time
	if t := cfg.Timestamp(); !t.IsZero() {
		end = t
	} else {
		end = time.Now()
	}
	s.span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))

	var m ptrace.ProtoMarshaler
	b, _ := m.MarshalTraces(s.traces) // TODO: do not ignore this error.

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

	event := s.span.Events().AppendEmpty()
	event.SetName(name)
	ts := pcommon.NewTimestampFromTime(tStamp)
	event.SetTimestamp(ts)
	setAttributes(event.Attributes(), attrs)
}

func (s *span) AddLink(link trace.Link) {
	if s == nil || !s.sampled {
		return
	}

	// TODO: handle link limits.

	addLinks(s.span.Links(), link)
}

func addLinks(dest ptrace.SpanLinkSlice, links ...trace.Link) {
	dest.EnsureCapacity(len(links))
	for _, link := range links {
		l := dest.AppendEmpty()
		l.SetTraceID(pcommon.TraceID(link.SpanContext.TraceID()))
		l.SetSpanID(pcommon.SpanID(link.SpanContext.SpanID()))
		l.SetFlags(uint32(link.SpanContext.TraceFlags()))
		l.TraceState().FromRaw(link.SpanContext.TraceState().String())
		setAttributes(l.Attributes(), link.Attributes)
	}
}

func (s *span) SetName(name string) {
	if s == nil || !s.sampled {
		return
	}
	s.span.SetName(name)
}

func (*span) TracerProvider() trace.TracerProvider { return GetTracerProvider() }
