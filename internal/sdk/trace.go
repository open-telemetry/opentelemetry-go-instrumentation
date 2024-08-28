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

func GetTracerProvider() trace.TracerProvider {
	return tracerProviderInstance
}

var tracerProviderInstance TracerProvider

type TracerProvider struct {
	embedded.TracerProvider
}

var _ trace.TracerProvider = TracerProvider{}

func (p TracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	cfg := trace.NewTracerConfig(opts...)
	return Tracer{
		name:      name,
		version:   cfg.InstrumentationVersion(),
		schemaURL: cfg.SchemaURL(),
	}
}

type Tracer struct {
	embedded.Tracer

	name, schemaURL, version string
}

var _ trace.Tracer = Tracer{}

func (t Tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	cfg := trace.NewSpanStartConfig(opts...)
	span := t.newSpan(ctx, name, cfg)
	ctx = trace.ContextWithSpan(ctx, span)
	return ctx, span
}

// newSpan returns a new configured span.
func (t *Tracer) newSpan(ctx context.Context, name string, cfg trace.SpanConfig) trace.Span {
	// If told explicitly to make this a new root use a zero value SpanContext
	// as a parent which contains an invalid trace ID and is not remote.
	var psc trace.SpanContext
	if cfg.NewRoot() {
		ctx = trace.ContextWithSpanContext(ctx, psc)
	} else {
		psc = trace.SpanContextFromContext(ctx)
	}

	span := &Span{sampled: true}
	t.sample(&span.sampled, parentStateFromContext(ctx))

	scc := trace.SpanContextConfig{TraceID: psc.TraceID()}
	idGen := getIDGenerator()
	scc.TraceID, scc.SpanID = idGen.Generate(scc.TraceID)
	putIDGenerator(idGen)

	if span.sampled {
		scc.TraceFlags = psc.TraceFlags() | trace.FlagsSampled

		span.traces, span.span = t.traces(ctx, name, cfg, scc, psc.SpanID())
	} else {
		scc.TraceFlags = psc.TraceFlags() &^ trace.FlagsSampled
	}
	span.spanContext = trace.NewSpanContext(scc)

	return span
}

type parentState uint8

const (
	emptyParent            parentState = 0
	parentRemoteSampled    parentState = 1
	parentRemoteNotSampled parentState = 2
	parentLocalSampled     parentState = 3
	parentLocalNotSampled  parentState = 4
)

func parentStateFromContext(ctx context.Context) parentState {
	psc := trace.SpanContextFromContext(ctx)
	if !psc.IsValid() {
		return emptyParent
	}

	if psc.IsRemote() {
		if psc.IsSampled() {
			return parentRemoteSampled
		}
		return parentRemoteNotSampled
	}

	if psc.IsSampled() {
		return parentLocalSampled
	}
	return parentLocalNotSampled
}

// Expected to be implemented in eBPF.
//
//go:noinline
func (t Tracer) sample(result *bool, ps parentState) {}

func (t Tracer) traces(ctx context.Context, name string, cfg trace.SpanConfig, scc trace.SpanContextConfig, psid trace.SpanID) (ptrace.Traces, ptrace.Span) {
	// TODO: pool this. It can be returned on end.
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)

	var rs ptrace.ResourceSpans
	if traces.ResourceSpans().Len() == 0 {
		rs = traces.ResourceSpans().AppendEmpty()
	} else {
		rs = traces.ResourceSpans().At(0)
	}
	rs.ScopeSpans().EnsureCapacity(1)

	var ss ptrace.ScopeSpans
	if rs.ScopeSpans().Len() == 0 {
		ss = rs.ScopeSpans().AppendEmpty()
	} else {
		ss = rs.ScopeSpans().At(0)
	}
	ss.Scope().SetName(t.name)
	ss.Scope().SetVersion(t.version)
	ss.SetSchemaUrl(t.schemaURL)
	ss.Spans().EnsureCapacity(1)

	var span ptrace.Span
	if ss.Spans().Len() == 0 {
		span = ss.Spans().AppendEmpty()
	} else {
		span = ss.Spans().At(0)
	}

	span.SetTraceID(pcommon.TraceID(scc.TraceID))
	span.SetSpanID(pcommon.SpanID(scc.SpanID))
	span.SetFlags(uint32(scc.TraceFlags))
	span.TraceState().FromRaw(scc.TraceState.String())
	span.SetParentSpanID(pcommon.SpanID(psid))
	span.SetName(name)
	span.SetKind(spanKind(cfg.SpanKind()))

	var start pcommon.Timestamp
	if t := cfg.Timestamp(); !t.IsZero() {
		start = pcommon.NewTimestampFromTime(cfg.Timestamp())
	} else {
		start = pcommon.NewTimestampFromTime(time.Now())
	}
	span.SetStartTimestamp(start)

	setAttributes(span.Attributes(), cfg.Attributes())
	addLinks(span.Links(), cfg.Links()...)

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

type Span struct {
	embedded.Span

	sampled     bool
	spanContext trace.SpanContext

	traces ptrace.Traces
	span   ptrace.Span
}

func (s *Span) SpanContext() trace.SpanContext {
	return s.spanContext
}

func (s *Span) IsRecording() bool {
	return s.sampled
}

func (s *Span) SetStatus(c codes.Code, msg string) {
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

func (s *Span) SetAttributes(attrs ...attribute.KeyValue) {
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

func (s *Span) End(opts ...trace.SpanEndOption) {
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
func (*Span) ended(buf []byte) {}

func (s *Span) RecordError(err error, opts ...trace.EventOption) {
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

func (s *Span) AddEvent(name string, opts ...trace.EventOption) {
	cfg := trace.NewEventConfig(opts...)
	s.addEvent(name, cfg.Timestamp(), cfg.Attributes())
}

func (s *Span) addEvent(name string, tStamp time.Time, attrs []attribute.KeyValue) {
	if tStamp.IsZero() {
		tStamp = time.Now()
	}

	// TODO: handle link limits.

	event := s.span.Events().AppendEmpty()
	event.SetName(name)
	event.SetTimestamp(pcommon.NewTimestampFromTime(tStamp))
	setAttributes(event.Attributes(), attrs)
}

func (s *Span) AddLink(link trace.Link) {
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

func (s *Span) SetName(name string) {
	if s == nil || !s.sampled {
		return
	}
	s.span.SetName(name)
}

func (*Span) TracerProvider() trace.TracerProvider { return GetTracerProvider() }
