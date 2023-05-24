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

package mux

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

func TestInstrumentorConvertEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Second)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	i := New()
	got := i.convertEvent(&Event{
		StartTime: uint64(start.UnixNano()),
		EndTime:   uint64(end.UnixNano()),
		// "GET"
		Method: [7]byte{0x47, 0x45, 0x54},
		// "/foo/bar"
		Path:        [100]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
		SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &events.Event{
		Library:     instrumentedPkg,
		Name:        "GET",
		Kind:        trace.SpanKindServer,
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String("GET"),
			semconv.HTTPTargetKey.String("/foo/bar"),
		},
	}
	assert.Equal(t, want, got)
}
