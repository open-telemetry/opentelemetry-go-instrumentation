// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestTraceIdEncoding(t *testing.T) {
	testCases := []struct {
		Name   string
		Input  TraceID
		Expect []byte
	}{
		{
			Name:   "trace id",
			Input:  TraceID{0x1},
			Expect: []byte(`"01000000000000000000000000000000"`),
		},
		{
			Name:   "empty trace id",
			Input:  TraceID{},
			Expect: []byte(`""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, runJSONEncodingTests(tc.Input, tc.Expect))
	}
}

func TestSpanIdEncoding(t *testing.T) {
	testCases := []struct {
		Name   string
		Input  SpanID
		Expect []byte
	}{
		{
			Name:   "span id",
			Input:  SpanID{0x1},
			Expect: []byte(`"0100000000000000"`),
		},
		{
			Name:   "empty span id",
			Input:  SpanID{},
			Expect: []byte(`""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, runJSONUnmarshalTest(tc.Input, tc.Expect))
	}
}
