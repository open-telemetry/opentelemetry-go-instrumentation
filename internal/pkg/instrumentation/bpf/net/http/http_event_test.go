// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // nolint:revive  // Internal package name.

import (
	"errors"
	"testing"
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
