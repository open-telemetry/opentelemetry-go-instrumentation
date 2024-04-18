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
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

func TestConvertEvent(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Second)
	hostString := "google.com"
	protoString := "1.1"
	methodString := "GET"
	pathString := "/home"
	var host [256]byte
	copy(host[:], hostString)
	var proto [8]byte
	copy(proto[:], protoString)
	var method [10]byte
	copy(method[:], methodString)
	var path [100]byte
	copy(path[:], pathString)

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
						semconv.URLPath(pathString),
						semconv.HTTPResponseStatusCodeKey.Int(200),
						semconv.ServerAddress("google.com"),
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
						semconv.URLPath(pathString),
						semconv.HTTPResponseStatusCodeKey.Int(400),
						semconv.ServerAddress("google.com"),
					},
					Status: probe.Status{Code: codes.Error},
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
