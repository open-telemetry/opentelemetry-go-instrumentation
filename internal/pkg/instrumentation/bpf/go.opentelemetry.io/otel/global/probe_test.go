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

	got := convertEvent(&event{
		BaseSpanProperties: context.BaseSpanProperties{
			StartTime:   uint64(start.UnixNano()),
			EndTime:     uint64(end.UnixNano()),
			SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
		},
		// span name: "Foo"
		SpanName: [64]byte{0x46, 0x6f, 0x6f},

		Attributes: attributesBuffer{
			Headers: [128]attributeHeader{
				{
					ValLength: 0,
					Vtype:     uint8(attribute.BOOL),
					Reserved:  0,
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.STRING),
					Reserved:  0,
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.FLOAT64),
					Reserved:  0,
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.INT64),
					Reserved:  0,
				},
				{
					ValLength: 0,
					Vtype:     uint8(attribute.STRING),
					Reserved:  0,
				},
			},
			// Keys are strings separated by null bytes: "bool_key\0string_key1\0float_key\0int_key\0string_key2\0"
			Keys: [256]byte{0x62, 0x6f, 0x6f, 0x6c, 0x5f, 0x6b, 0x65, 0x79, 0x00, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6b, 0x65, 0x79, 0x31, 0x00, 0x66, 0x6c, 0x6f, 0x61, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x00, 0x69, 0x6e, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x00, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6b, 0x65, 0x79, 0x32, 0x00},
			// Numberc values of: true, 3.14(pi), 42
			NumericValues: [32]int64{1, int64(math.Float64bits(math.Pi)), 42},
			// String values of: "string value 1", "string value 2", saved as null terminated strings
			StrValues: [1024]byte{0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x31, 0x00, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x20, 0x32, 0x00},
		},
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &probe.Event{
		Package:     pkg,
		Name:        "Foo",
		Kind:        trace.SpanKindClient,
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
	assert.Equal(t, want, got)
}
