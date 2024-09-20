// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := utils.TimeToBootOffset(start)
	endOffset := utils.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	got := convertEvent(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   startOffset,
			EndTime:     endOffset,
			SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
		},
		// "SELECT * FROM foo"
		Query: [256]byte{0x53, 0x45, 0x4c, 0x45, 0x43, 0x54, 0x20, 0x2a, 0x20, 0x46, 0x52, 0x4f, 0x4d, 0x20, 0x66, 0x6f, 0x6f},
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &probe.SpanEvent{
		SpanName:    "DB",
		StartTime:   start,
		EndTime:     end,
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.DBQueryText("SELECT * FROM foo"),
		},
		TracerSchema: semconv.SchemaURL,
	}
	assert.Equal(t, want, got[0])
}
