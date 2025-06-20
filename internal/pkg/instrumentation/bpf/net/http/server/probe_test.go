// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/pdataconv"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestProbeConvertEvent(t *testing.T) {
	start := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	end := start.Add(1 * time.Second)

	startOffset := utils.TimeToBootOffset(start)
	endOffset := utils.TimeToBootOffset(end)

	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}

	testCases := []struct {
		name     string
		event    *event
		expected ptrace.SpanSlice
	}{
		{
			name: "basic server test",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startOffset,
					EndTime:     endOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 200,
				// "GET"
				Method: [8]byte{0x47, 0x45, 0x54},
				// "/foo/bar"
				Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
				// "www.google.com:8080"
				RemoteAddr: [256]byte{
					0x77, 0x77, 0x77, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
					0x2e, 0x63, 0x6f, 0x6d, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "localhost:8080"
				Host: [256]byte{
					0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x3a,
					0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "HTTP/1.1"
				Proto: [8]byte{0x48, 0x54, 0x54, 0x50, 0x2f, 0x31, 0x2e, 0x31},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("GET")
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				pdataconv.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/foo/bar"),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.NetworkPeerAddress("www.google.com"),
					semconv.NetworkPeerPort(8080),
					semconv.ServerAddress("localhost"),
					semconv.ServerPort(8080),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			name: "proto name added when not HTTP",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startOffset,
					EndTime:     endOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 200,
				// "GET"
				Method: [8]byte{0x47, 0x45, 0x54},
				// "/foo/bar"
				Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
				// "www.google.com:8080"
				RemoteAddr: [256]byte{
					0x77, 0x77, 0x77, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
					0x2e, 0x63, 0x6f, 0x6d, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "localhost:8080"
				Host: [256]byte{
					0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x3a,
					0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "FOO/2.2"
				Proto: [8]byte{0x46, 0x4f, 0x4f, 0x2f, 0x32, 0x2e, 0x32},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("GET")
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				pdataconv.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/foo/bar"),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.NetworkPeerAddress("www.google.com"),
					semconv.NetworkPeerPort(8080),
					semconv.ServerAddress("localhost"),
					semconv.ServerPort(8080),
					semconv.NetworkProtocolName("FOO"),
					semconv.NetworkProtocolVersion("2.2"),
				)

				return spans
			}(),
		},
		{
			name: "server statuscode 400 doesn't set span.Status",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startOffset,
					EndTime:     endOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 400,
				// "GET"
				Method: [8]byte{0x47, 0x45, 0x54},
				// "/foo/bar"
				Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
				// "www.google.com:8080"
				RemoteAddr: [256]byte{
					0x77, 0x77, 0x77, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
					0x2e, 0x63, 0x6f, 0x6d, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "localhost:8080"
				Host: [256]byte{
					0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x3a,
					0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "HTTP/1.1"
				Proto: [8]byte{0x48, 0x54, 0x54, 0x50, 0x2f, 0x31, 0x2e, 0x31},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("GET")
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				pdataconv.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/foo/bar"),
					semconv.HTTPResponseStatusCodeKey.Int(400),
					semconv.NetworkPeerAddress("www.google.com"),
					semconv.NetworkPeerPort(8080),
					semconv.ServerAddress("localhost"),
					semconv.ServerPort(8080),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			name: "server statuscode 500 sets span.Status",
			event: &event{
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startOffset,
					EndTime:     endOffset,
					SpanContext: context.EBPFSpanContext{TraceID: traceID, SpanID: spanID},
				},
				StatusCode: 500,
				// "GET"
				Method: [8]byte{0x47, 0x45, 0x54},
				// "/foo/bar"
				Path: [128]byte{0x2f, 0x66, 0x6f, 0x6f, 0x2f, 0x62, 0x61, 0x72},
				// "www.google.com:8080"
				RemoteAddr: [256]byte{
					0x77, 0x77, 0x77, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
					0x2e, 0x63, 0x6f, 0x6d, 0x3a, 0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "localhost:8080"
				Host: [256]byte{
					0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x3a,
					0x38, 0x30, 0x38, 0x30, 0x0,
				},
				// "HTTP/1.1"
				Proto: [8]byte{0x48, 0x54, 0x54, 0x50, 0x2f, 0x31, 0x2e, 0x31},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName("GET")
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(utils.BootOffsetToTimestamp(startOffset))
				span.SetEndTimestamp(utils.BootOffsetToTimestamp(endOffset))
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetFlags(uint32(trace.FlagsSampled))
				span.Status().SetCode(ptrace.StatusCodeError)
				pdataconv.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String("GET"),
					semconv.URLPath("/foo/bar"),
					semconv.HTTPResponseStatusCodeKey.Int(500),
					semconv.NetworkPeerAddress("www.google.com"),
					semconv.NetworkPeerPort(8080),
					semconv.ServerAddress("localhost"),
					semconv.ServerPort(8080),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			out := processFn(tt.event)
			assert.Equal(t, tt.expected, out)
		})
	}
}
