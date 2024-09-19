// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	// TODO implement.
	s := &span{}
	s.traces, s.span = t.traces(ctx, "", trace.SpanConfig{}, trace.SpanContext{}, trace.SpanContext{})
	t.start(ctx, s, nil, nil, nil)
	return ctx, s
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
}

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
	// TODO: configure span.
	return traces, span
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
	/* TODO: implement */
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
	// TODO: implement.
	s.ended(nil)
}

// Expected to be implemented in eBPF.
//
//go:noinline
func (*span) ended(buf []byte) {}

func (s *span) RecordError(err error, opts ...trace.EventOption) {
	if s == nil || err == nil || !s.sampled {
		return
	}
	/* TODO: implement */
}

func (s *span) AddEvent(name string, opts ...trace.EventOption) {
	if s == nil || !s.sampled {
		return
	}
	/* TODO: implement */
}

func (s *span) AddLink(link trace.Link) {
	if s == nil || !s.sampled {
		return
	}
	/* TODO: implement */
}

func (s *span) SetName(name string) {
	if s == nil || !s.sampled {
		return
	}
	/* TODO: implement */
}

func (*span) TracerProvider() trace.TracerProvider {
	return GetTracerProvider()
}
