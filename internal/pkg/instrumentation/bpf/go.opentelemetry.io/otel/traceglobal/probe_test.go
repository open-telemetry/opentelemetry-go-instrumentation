// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package global

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := utils.TimeToBootOffset(start)
	endOffset := utils.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	var floatBuf [128]byte
	binary.LittleEndian.PutUint64(floatBuf[:], math.Float64bits(math.Pi))

	got := processFn(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   startOffset,
			EndTime:     endOffset,
			SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
		},
		// span name: "Foo"
		SpanName: [64]byte{0x46, 0x6f, 0x6f},

		Attributes: attributesBuffer{
			AttrsKv: [16]attributeKeyVal{
				{
					ValLength: 0,
					Vtype:     uint8(attribute.BOOL),
					Reserved:  0,
					// "bool_key"
					Key: [32]byte{0x62, 0x6f, 0x6f, 0x6c, 0x5f, 0x6b, 0x65, 0x79},
					// true
					Value: [128]byte{0x01},
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.STRING),
					Reserved:  0,
					// "string_key1"
					Key: [32]byte{0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6b, 0x65, 0x79, 0x31},
					// "string value 1"
					Value: [128]byte{
						0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20,
						0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x31,
					},
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.FLOAT64),
					Reserved:  0,
					// "float_key"
					Key: [32]byte{0x66, 0x6c, 0x6f, 0x61, 0x74, 0x5f, 0x6b, 0x65, 0x79},
					// math.Pi
					Value: floatBuf,
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.INT64),
					Reserved:  0,
					// "int_key"
					Key: [32]byte{0x69, 0x6e, 0x74, 0x5f, 0x6b, 0x65, 0x79},
					// 42
					Value: [128]byte{42},
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.STRING),
					Reserved:  0,
					// "string_key2"
					Key: [32]byte{0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6b, 0x65, 0x79, 0x32},
					// "string value 2"
					Value: [128]byte{
						0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20,
						0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x32,
					},
				},
			},
			ValidAttrs: 5,
		},
		TracerID: tracerID{
			// "user-tracer"
			Name: [128]byte{0x75, 0x73, 0x65, 0x72, 0x2d, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72},
			// "v1"
			Version: [32]byte{0x76, 0x31},
			// "user-schema"
			SchemaURL: [128]byte{0x75, 0x73, 0x65, 0x72, 0x2d, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61},
		},
	})

	want := func() ptrace.ScopeSpans {
		ss := ptrace.NewScopeSpans()

		ss.Scope().SetName("user-tracer")
		ss.Scope().SetVersion("v1")
		ss.SetSchemaUrl("user-schema")

		span := ss.Spans().AppendEmpty()
		span.SetName("Foo")
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
		span.SetTraceID(pcommon.TraceID(traceID))
		span.SetSpanID(pcommon.SpanID(spanID))
		span.SetFlags(uint32(trace.FlagsSampled))
		utils.Attributes(
			span.Attributes(),
			attribute.Bool("bool_key", true),
			attribute.String("string_key1", "string value 1"),
			attribute.Float64("float_key", math.Pi),
			attribute.Int64("int_key", 42),
			attribute.String("string_key2", "string value 2"),
		)

		return ss
	}()
	assert.Equal(t, want, got)
}
