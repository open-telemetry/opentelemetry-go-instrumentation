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

package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestConvertEvent(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Second)
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
	spanContext := trace.NewSpanContext(
		trace.SpanContextConfig{
			SpanID:     spId,
			TraceID:    trId,
			TraceFlags: 1,
		},
	)

	testCases := []struct {
		name     string
		event    *event
		expected []*probe.SpanEvent
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.URLPath(pathString),
						semconv.URLFull("http://google.com/home"),
						semconv.ServerAddress(hostString),
						semconv.NetworkProtocolVersion("1.1"),
					},
				},
			},
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(400),
						semconv.URLPath(pathString),
						semconv.URLFull("http://google.com/home"),
						semconv.ServerAddress(hostString),
						semconv.NetworkProtocolVersion("1.1"),
					},
					Status: probe.Status{Code: codes.Error},
				},
			},
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(500),
						semconv.URLPath(pathString),
						semconv.URLFull("http://google.com/home"),
						semconv.ServerAddress(hostString),
						semconv.NetworkProtocolVersion("1.1"),
					},
					Status: probe.Status{Code: codes.Error},
				},
			},
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.URLPath(pathString),
						semconv.URLFull("foo://google.com/home"),
						semconv.ServerAddress(hostString),
						semconv.NetworkProtocolName("foo"),
						semconv.NetworkProtocolVersion("2.2"),
					},
				},
			},
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.URLPath(pathString),
						semconv.URLFull("http://user@google.com/home?query=true#fragment"),
						semconv.ServerAddress(hostString),
						semconv.NetworkProtocolVersion("1.1"),
					},
				},
			},
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
					StartTime:   uint64(startTime.Unix()),
					EndTime:     uint64(endTime.Unix()),
					SpanContext: context.EBPFSpanContext{TraceID: trId, SpanID: spId},
				},
			},
			expected: []*probe.SpanEvent{
				{
					SpanName:    methodString,
					SpanContext: &spanContext,
					StartTime:   startTime.Unix(),
					EndTime:     endTime.Unix(),
					Attributes: []attribute.KeyValue{
						semconv.HTTPRequestMethodKey.String(methodString),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.URLPath(pathString),
						semconv.URLFull("http:/home?"),
						semconv.NetworkProtocolVersion("1.1"),
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			out := convertEvent(tt.event)
			assert.Equal(t, tt.expected, out)
		})
	}
}
