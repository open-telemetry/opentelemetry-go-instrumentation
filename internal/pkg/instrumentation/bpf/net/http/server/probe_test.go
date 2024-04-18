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
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Second)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	testCases := []struct{
		name string
		event *event
		expected []*probe.SpanEvent
	}{
		{
			name: "basic server test",
			event: &event{
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
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    "GET",
					StartTime:   int64(start.UnixNano()),
					EndTime:     int64(end.UnixNano()),
					SpanContext: &sc,
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String("GET"),
						semconv.URLPath("/foo/bar"),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.NetworkPeerAddress("www.google.com"),
						semconv.NetworkPeerPort(8080),
						semconv.ServerAddress("localhost"),
						semconv.ServerPort(8080),
						semconv.NetworkProtocolVersion("1.1"),
					},
				},
			},
		},
		{
			name: "server statuscode 400 doesn't set span.Status",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   uint64(start.UnixNano()),
					EndTime:     uint64(end.UnixNano()),
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 400,
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
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    "GET",
					StartTime:   int64(start.UnixNano()),
					EndTime:     int64(end.UnixNano()),
					SpanContext: &sc,
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String("GET"),
						semconv.URLPath("/foo/bar"),
						semconv.HTTPResponseStatusCodeKey.Int(400),
						semconv.NetworkPeerAddress("www.google.com"),
						semconv.NetworkPeerPort(8080),
						semconv.ServerAddress("localhost"),
						semconv.ServerPort(8080),
						semconv.NetworkProtocolVersion("1.1"),
					},
				},
			},
		},
		{
			name: "server statuscode 500 sets span.Status",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   uint64(start.UnixNano()),
					EndTime:     uint64(end.UnixNano()),
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 500,
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
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    "GET",
					StartTime:   int64(start.UnixNano()),
					EndTime:     int64(end.UnixNano()),
					SpanContext: &sc,
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String("GET"),
						semconv.URLPath("/foo/bar"),
						semconv.HTTPResponseStatusCodeKey.Int(500),
						semconv.NetworkPeerAddress("www.google.com"),
						semconv.NetworkPeerPort(8080),
						semconv.ServerAddress("localhost"),
						semconv.ServerPort(8080),
						semconv.NetworkProtocolVersion("1.1"),
					},
					Status: probe.Status{Code: codes.Error},
				},
			},
		},
	}

	for _, tt := range testCases{
		t.Run(tt.name, func(t *testing.T) {
			out := convertEvent(tt.event)
			assert.Equal(t, tt.expected, out)
		})
	}
}
