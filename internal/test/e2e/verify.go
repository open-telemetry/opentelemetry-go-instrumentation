// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"testing" // nolint:depguard  // This is a testing utility package.

	"github.com/stretchr/testify/assert" // nolint:depguard  // This is a testing utility package.
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// PortRE is a regular expression that matches valid port numbers.
var PortRE = regexp.MustCompile(`^[1-9]\d{1,4}$`)

// ResourceSpans returns all resource spans in the given trace data.
func ResourceSpans(td ptrace.Traces) []ptrace.ResourceSpans {
	var spans []ptrace.ResourceSpans
	rs := td.ResourceSpans()
	for i := range rs.Len() {
		spans = append(spans, rs.At(i))
	}
	return spans
}

// ResourceAttribute returns the value of a specific resource attribute
// if found.
func ResourceAttribute(td ptrace.Traces, key string) (pcommon.Value, error) {
	for _, rs := range ResourceSpans(td) {
		attrs := rs.Resource().Attributes()
		if val, ok := attrs.Get(key); ok {
			return val, nil
		}
	}
	return pcommon.NewValueEmpty(), fmt.Errorf("resource attribute %q not found", key)
}

// ScopeSpansByName filters scope spans matching the provided scope
// name.
func ScopeSpansByName(td ptrace.Traces, name string) []ptrace.ScopeSpans {
	var result []ptrace.ScopeSpans
	for _, rs := range ResourceSpans(td) {
		scopes := rs.ScopeSpans()
		for i := range scopes.Len() {
			ss := scopes.At(i)
			if ss.Scope().Name() == name {
				result = append(result, ss)
			}
		}
	}
	return result
}

// SelectSpan returns the first span matching the selector from a set of scope
// spans.
func SelectSpan(
	scopeSpans []ptrace.ScopeSpans,
	selector func(ptrace.Span) bool,
) (ptrace.Span, error) {
	for _, ss := range scopeSpans {
		spans := ss.Spans()
		for i := range spans.Len() {
			span := spans.At(i)
			if selector(span) {
				return span, nil
			}
		}
	}
	return ptrace.NewSpan(), errors.New("span not found")
}

// SpanByName returns the first span with the specified name from a
// set of scope spans.
func SpanByName(scopeSpans []ptrace.ScopeSpans, name string) (ptrace.Span, error) {
	for _, ss := range scopeSpans {
		spans := ss.Spans()
		for i := range spans.Len() {
			span := spans.At(i)
			if span.Name() == name {
				return span, nil
			}
		}
	}
	return ptrace.NewSpan(), fmt.Errorf("span %q not found", name)
}

// AttributesMap converts the given attribute map to a native Go map.
func AttributesMap(attrs pcommon.Map) map[string]pcommon.Value {
	result := make(map[string]pcommon.Value)
	attrs.Range(func(k string, v pcommon.Value) bool {
		result[k] = v
		return true
	})
	return result
}

// EventByName finds the first event in the span matching the given
// name.
func EventByName(span ptrace.Span, name string) (ptrace.SpanEvent, error) {
	events := span.Events()
	for i := range events.Len() {
		event := events.At(i)
		if event.Name() == name {
			return event, nil
		}
	}
	return ptrace.NewSpanEvent(), fmt.Errorf("event %q not found", name)
}

// LinkByTraceAndSpanID locates a link in the span matching the given
// trace and span ID.
func LinkByTraceAndSpanID(span ptrace.Span, traceID, spanID string) (ptrace.SpanLink, error) {
	links := span.Links()
	for i := range links.Len() {
		link := links.At(i)
		tidB := [16]byte(link.TraceID())
		tid := hex.EncodeToString(tidB[:])
		sidB := [8]byte(link.SpanID())
		sid := hex.EncodeToString(sidB[:])
		if tid == traceID && sid == spanID {
			return link, nil
		}
	}
	return ptrace.NewSpanLink(), errors.New("link not found")
}

var traceIDRe = regexp.MustCompile(`^[a-f0-9]{32}$`)

func AssertTraceID(t *testing.T, traceID pcommon.TraceID, msgAndArgs ...any) bool {
	tidB := [16]byte(traceID)
	tid := hex.EncodeToString(tidB[:])
	return assert.Regexp(t, traceIDRe, tid, msgAndArgs...)
}

var spanIDRe = regexp.MustCompile(`^[a-f0-9]{16}$`)

func AssertSpanID(t *testing.T, spanID pcommon.SpanID, msgAndArgs ...any) bool {
	sidB := [8]byte(spanID)
	sid := hex.EncodeToString(sidB[:])
	return assert.Regexp(t, spanIDRe, sid, msgAndArgs...)
}
