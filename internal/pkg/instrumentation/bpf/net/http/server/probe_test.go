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

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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
		StatusCode: 200,
		// "GET"
		Method: [8]byte{0x47, 0x45, 0x54},
		// "/foo/bar"
		Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
		// "www.google.com:8080"
		RemoteAddr: [256]byte{0x77, 0x77, 0x77, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0},
		// "localhost:8080"
		Host: [256]byte{0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0},
		// "HTTP/1.1"
		Proto: [8]byte{0x48, 0x54, 0x54, 0x50, 0x2f, 0x31, 0x2e, 0x31},
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	want := &probe.Event{
		Package:     pkg,
		Name:        "GET",
		Kind:        trace.SpanKindServer,
		StartTime:   int64(start.UnixNano()),
		EndTime:     int64(end.UnixNano()),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String("GET"),
			semconv.HTTPTargetKey.String("/foo/bar"),
			semconv.HTTPStatusCodeKey.Int(200),
			semconv.NetHostName("localhost:8080"),
			semconv.NetProtocolName("HTTP/1.1"),
			semconv.NetPeerName("www.google.com"),
			semconv.NetPeerPort(8080),
		},
	}
	assert.Equal(t, want, got)
}
