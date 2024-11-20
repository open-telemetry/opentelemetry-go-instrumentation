// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.opentelemetry.io/auto/sdk/internal/telemetry"
)

type span struct {
	noop.Span

	spanContext trace.SpanContext
	sampled     atomic.Bool

	mu     sync.Mutex
	traces *telemetry.Traces
	span   *telemetry.Span
}

func (s *span) SpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}
	// s.spanContext is immutable, do not acquire lock s.mu.
	return s.spanContext
}

func (s *span) IsRecording() bool {
	if s == nil {
		return false
	}

	return s.sampled.Load()
}

func (s *span) SetStatus(c codes.Code, msg string) {
	if s == nil || !s.sampled.Load() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s == nil || !s.sampled.Load() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s == nil || !s.sampled.Swap(false) {
		return
	}

	// s.end exists so the lock (s.mu) is not held while s.ended is called.
	s.ended(s.end(opts))
}

func (s *span) end(opts []trace.SpanEndOption) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := trace.NewSpanEndConfig(opts...)
	if t := cfg.Timestamp(); !t.IsZero() {
		s.span.EndTime = cfg.Timestamp()
	} else {
		s.span.EndTime = time.Now()
	}

	b, _ := json.Marshal(s.traces) // TODO: do not ignore this error.
	return b
}

// Expected to be implemented in eBPF.
//
//go:noinline
func (*span) ended(buf []byte) { ended(buf) }

// ended is used for testing.
var ended = func([]byte) {}

func (s *span) RecordError(err error, opts ...trace.EventOption) {
	if s == nil || err == nil || !s.sampled.Load() {
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

	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s == nil || !s.sampled.Load() {
		return
	}

	cfg := trace.NewEventConfig(opts...)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.addEvent(name, cfg.Timestamp(), cfg.Attributes())
}

// addEvent adds an event with name and attrs at tStamp to the span. The span
// lock (s.mu) needs to be held by the caller.
func (s *span) addEvent(name string, tStamp time.Time, attrs []attribute.KeyValue) {
	// TODO: handle event limits.

	s.span.Events = append(s.span.Events, &telemetry.SpanEvent{
		Time:  tStamp,
		Name:  name,
		Attrs: convAttrs(attrs),
	})
}

func (s *span) AddLink(link trace.Link) {
	if s == nil || !s.sampled.Load() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s == nil || !s.sampled.Load() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.span.Name = name
}

func (*span) TracerProvider() trace.TracerProvider { return TracerProvider() }
