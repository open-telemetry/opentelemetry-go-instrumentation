// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package global

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Second)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	var floatBuf [128]byte
	binary.LittleEndian.PutUint64(floatBuf[:], math.Float64bits(math.Pi))

	got := convertEvent(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   uint64(start.UnixNano()),
			EndTime:     uint64(end.UnixNano()),
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
					Value: [128]byte{0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x31},
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
					Value: [128]byte{0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x32},
				},
			},
			ValidAttrs: 5,
		},
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &probe.SpanEvent{
		SpanName:    "Foo",
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			attribute.Bool("bool_key", true),
			attribute.String("string_key1", "string value 1"),
			attribute.Float64("float_key", math.Pi),
			attribute.Int64("int_key", 42),
			attribute.String("string_key2", "string value 2"),
		},
	}
	assert.Equal(t, want, got[0])
}
