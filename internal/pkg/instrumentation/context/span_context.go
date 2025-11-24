// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package context contains tracing types used among probes.
package context // nolint:revive  // Internal package name.

import "go.opentelemetry.io/otel/trace"

// BaseSpanProperties contains the basic attributes filled by all probes.
type BaseSpanProperties struct {
	StartTime         uint64
	EndTime           uint64
	SpanContext       EBPFSpanContext
	ParentSpanContext EBPFSpanContext
}

// EBPFSpanContext is the the span context representation within the eBPF
// instrumentation system.
type EBPFSpanContext struct {
	TraceID    trace.TraceID
	SpanID     trace.SpanID
	TraceFlags trace.TraceFlags
	_          [7]byte // padding
}
