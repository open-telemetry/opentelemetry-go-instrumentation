// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // nolint:revive  // Internal package name.

import (
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// TestParsePattern tests the ParsePattern function with various inputs.
func TestParsePattern(t *testing.T) {
	// Define test cases
	tests := []struct {
		name    string
		input   string
		path    string
		wantErr error
	}{
		{
			name:    "Normal case",
			input:   "GET example.com/test/{id}",
			path:    "/test/{id}",
			wantErr: nil,
		},
		{
			name:    "No method",
			input:   "example.com/test",
			path:    "/test",
			wantErr: nil,
		},
		{
			name:    "Empty input",
			input:   "",
			path:    "",
			wantErr: ErrEmptyPattern,
		},
		{
			name:    "Missing path or host",
			input:   "GET example.com",
			path:    "",
			wantErr: ErrMissingPathOrHost,
		},
		{
			name:    "Simple / path with host",
			input:   "GET example.com/",
			path:    "/",
			wantErr: nil,
		},
		{
			name:    "Simple / path without host",
			input:   "GET /",
			path:    "/",
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, err := ParsePattern(tc.input)

			if path != tc.path || !errors.Is(err, tc.wantErr) {
				t.Errorf("TestParsePattern(%q) = %q, %v; want %q, %v",
					tc.input, path, err, tc.path, tc.wantErr)
			}
		})
	}
}

func TestServerAddressPortAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAddr attribute.KeyValue
		wantPort attribute.KeyValue
	}{
		{
			name:     "Host only",
			input:    "example.com",
			wantAddr: semconv.ServerAddress("example.com"),
		},
		{
			name:     "Host and port",
			input:    "example.com:8080",
			wantAddr: semconv.ServerAddress("example.com"),
			wantPort: semconv.ServerPort(8080),
		},
		{
			name:     "IPv6 host and port",
			input:    "[::1]:8080",
			wantAddr: semconv.ServerAddress("::1"),
			wantPort: semconv.ServerPort(8080),
		},
		{
			name:     "IPv6 host only",
			input:    "[::1]",
			wantAddr: semconv.ServerAddress("::1"),
		},
		{
			name:     "IPv6 global host only",
			input:    "[2001:db8::1]",
			wantAddr: semconv.ServerAddress("2001:db8::1"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, port := ServerAddressPortAttributes([]byte(tc.input))

			if addr != tc.wantAddr || port != tc.wantPort {
				t.Errorf("ServerAddressPortAttributes(%q) = %v, %v; want %v, %v",
					tc.input, addr, port, tc.wantAddr, tc.wantPort)
			}
		})
	}
}
