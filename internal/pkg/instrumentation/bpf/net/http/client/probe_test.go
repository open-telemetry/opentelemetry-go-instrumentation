// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestConvertEvent(t *testing.T) {
	startTime := time.Unix(0, time.Now().UnixNano()) // No wall clock.
	endTime := startTime.Add(1 * time.Second)

	startTimeOffset := utils.TimeToBootOffset(startTime)
	endTimeOffset := utils.TimeToBootOffset(endTime)

	hostString := "google.com"
	protoString := "HTTP/1.1"
	protoFooString := "foo/2.2"
	methodString := "GET"
	pathString := "/home"
	schemeString := "http"
	fooSchemeString := "foo"
	opaqueString := "foobar"
	rawPathString := "rawpath"
	usernameString := "user"
	rawQueryString := "query=true"
	fragmentString := "fragment"
	rawFragmentString := "#rawFragment"
	var host [128]byte
	copy(host[:], hostString)
	var proto [8]byte
	copy(proto[:], protoString)
	var protoFoo [8]byte
	copy(protoFoo[:], protoFooString)
	var method [16]byte
	copy(method[:], methodString)
	var path [128]byte
	copy(path[:], pathString)
	var scheme [8]byte
	copy(scheme[:], schemeString)
	var fooScheme [8]byte
	copy(fooScheme[:], fooSchemeString)
	var opaque [8]byte
	copy(opaque[:], opaqueString)
	var rawPath [8]byte
	copy(rawPath[:], []byte(rawPathString))
	var username [8]byte
	copy(username[:], []byte(usernameString))
	var rawQuery [128]byte
	copy(rawQuery[:], []byte(rawQueryString))
	var fragment [56]byte
	copy(fragment[:], []byte(fragmentString))
	var rawFragment [56]byte
	copy(rawFragment[:], []byte(rawFragmentString))

	spId, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	assert.NoError(t, err)
	trId, err := trace.TraceIDFromHex("00f067aa0ba902b700f067aa0ba902b7")
	assert.NoError(t, err)

	testCases := []struct {
		name     string
		event    *event
		expected ptrace.SpanSlice
	}{
		{
			name: "basic client event",
			event: &event{
				Host:       host,
				Proto:      proto,
				StatusCode: uint64(200),
				Method:     method,
				Path:       path,
				Scheme:     scheme,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.URLPath(pathString),
					semconv.URLFull("http://google.com/home"),
					semconv.ServerAddress(hostString),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			name: "client event code 400",
			event: &event{
				Host:       host,
				Proto:      proto,
				StatusCode: uint64(400),
				Method:     method,
				Path:       path,
				Scheme:     scheme,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
				span.Status().SetCode(ptrace.StatusCodeError)

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(400),
					semconv.URLPath(pathString),
					semconv.URLFull("http://google.com/home"),
					semconv.ServerAddress(hostString),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			name: "client event code 500",
			event: &event{
				Host:       host,
				Proto:      proto,
				StatusCode: uint64(500),
				Method:     method,
				Path:       path,
				Scheme:     scheme,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
				span.Status().SetCode(ptrace.StatusCodeError)

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(500),
					semconv.URLPath(pathString),
					semconv.URLFull("http://google.com/home"),
					semconv.ServerAddress(hostString),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			name: "non-http protocol.name",
			event: &event{
				Host:       host,
				Proto:      protoFoo,
				StatusCode: uint64(200),
				Method:     method,
				Path:       path,
				Scheme:     fooScheme,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.URLPath(pathString),
					semconv.URLFull("foo://google.com/home"),
					semconv.ServerAddress(hostString),
					semconv.NetworkProtocolName("foo"),
					semconv.NetworkProtocolVersion("2.2"),
				)

				return spans
			}(),
		},
		{
			name: "basic url parsing",
			event: &event{
				Host:       host,
				Proto:      proto,
				StatusCode: uint64(200),
				Method:     method,
				Path:       path,
				Scheme:     scheme,
				Username:   username,
				RawQuery:   rawQuery,
				Fragment:   fragment,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.URLPath(pathString),
					semconv.URLFull("http://user@google.com/home?query=true#fragment"),
					semconv.ServerAddress(hostString),
					semconv.NetworkProtocolVersion("1.1"),
				)

				return spans
			}(),
		},
		{
			// see https://cs.opensource.google/go/go/+/refs/tags/go1.22.2:src/net/url/url.go;l=815
			name: "url parsing with ForceQuery (includes '?' without query value) and OmitHost (does not write '//' with empty host and username)",
			event: &event{
				Host:       [128]byte{},
				Proto:      proto,
				StatusCode: uint64(200),
				Method:     method,
				Path:       path,
				Scheme:     scheme,
				RawQuery:   [128]byte{},
				ForceQuery: 1,
				OmitHost:   1,
				BaseSpanProperties: context.BaseSpanProperties{
					StartTime:   startTimeOffset,
					EndTime:     endTimeOffset,
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: func() ptrace.SpanSlice {
				spans := ptrace.NewSpanSlice()
				span := spans.AppendEmpty()
				span.SetName(methodString)
				span.SetKind(ptrace.SpanKindClient)
				span.SetTraceID(pcommon.TraceID(trId))
				span.SetSpanID(pcommon.SpanID(spId))
				span.SetFlags(1)
				span.SetKind(ptrace.SpanKindClient)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

				utils.Attributes(
					span.Attributes(),
					semconv.HTTPRequestMethodKey.String(methodString),
					semconv.HTTPResponseStatusCodeKey.Int(200),
					semconv.URLPath(pathString),
					semconv.URLFull("http:/home?"),
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
