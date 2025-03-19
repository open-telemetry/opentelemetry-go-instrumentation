// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type eBPFEventKeyType struct{}

var eBPFEventKey eBPFEventKeyType

// contextWithSpan returns a derived copy of parent that contains span.
func contextWithSpan(parent context.Context, span ptrace.Span) context.Context {
	return context.WithValue(parent, eBPFEventKey, span)
}

// spanFromContext returns the Span within ctx and true if one exists.
// Otherwise, an empty (i.e. invalid) span and false are returned.
func spanFromContext(ctx context.Context) (ptrace.Span, bool) {
	val := ctx.Value(eBPFEventKey)
	if val == nil {
		// Do not allocate underlying OTLP types by returning:
		//
		//   return ptrace.NewSpan(), false
		return ptrace.Span{}, false
	}

	s, ok := val.(ptrace.Span)
	return s, ok
}
