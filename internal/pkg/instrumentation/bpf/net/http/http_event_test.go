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

package http

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
		method  string
		host    string
		path    string
		wantErr error
	}{
		{
			name:    "Normal case",
			input:   "GET example.com/test/{id}",
			method:  "GET",
			host:    "example.com",
			path:    "/test/{id}",
			wantErr: nil,
		},
		{
			name:    "No method",
			input:   "example.com/test",
			method:  "",
			host:    "example.com",
			path:    "/test",
			wantErr: nil,
		},
		{
			name:    "Empty input",
			input:   "",
			method:  "",
			host:    "",
			path:    "",
			wantErr: ErrEmptyPattern,
		},
		{
			name:    "Missing path or host",
			input:   "GET example.com",
			method:  "",
			host:    "",
			path:    "",
			wantErr: ErrMissingPathOrHost,
		},
		{
			name:    "Simple / path with host",
			input:   "GET example.com/",
			method:  "GET",
			host:    "example.com",
			path:    "/",
			wantErr: nil,
		},
		{
			name:    "Simple / path without host",
			input:   "GET /",
			method:  "GET",
			host:    "",
			path:    "/",
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method, host, path, err := ParsePattern(tc.input)

			if method != tc.method || host != tc.host || path != tc.path || !errors.Is(err, tc.wantErr) {
				t.Errorf("TestParsePattern(%q) = %q, %q, %q, %v; want %q, %q, %q, %v",
					tc.input, method, host, path, err, tc.method, tc.host, tc.path, tc.wantErr)
			}
		})
	}
}
