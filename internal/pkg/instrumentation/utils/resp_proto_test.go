// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"strings"
	"testing"
)

// TestParseNextResp tests the parseNextResp function which parses a single RESP command
// from the beginning of the input string.
func TestParseNextResp(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantSegCount  int
		wantErr       bool
		wantCmdPrefix string // optional check to see if parsed cmd has a certain prefix
	}{
		{
			name:          "Valid single command - 2 segments",
			input:         "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
			wantSegCount:  2,
			wantErr:       false,
			wantCmdPrefix: "*2\r\n",
		},
		{
			name:          "Valid single command - 3 segments",
			input:         "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
			wantSegCount:  3,
			wantErr:       false,
			wantCmdPrefix: "*3\r\n",
		},
		{
			name:         "Missing '*' at the start",
			input:        "2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
			wantSegCount: 0,
			wantErr:      true,
		},
		{
			name:         "Missing CRLF after array count",
			input:        "*3$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
			wantSegCount: 0,
			wantErr:      true,
		},
		{
			name:         "Invalid bulk length",
			input:        "*1\r\n$xyz\r\nabcdef\r\n",
			wantSegCount: 0,
			wantErr:      true,
		},
		{
			name:         "Not enough data for bulk string content",
			input:        "*1\r\n$5\r\nhi\r\n", // bulkLen=5 but only 2 bytes "hi"
			wantSegCount: 0,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			cmd, segCount, leftover, err := parseNextResp(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseNextResp() error = %v, wantErr = %v", err, tc.wantErr)
				return
			}

			if segCount != tc.wantSegCount {
				t.Errorf("parseNextResp() segCount = %d, want = %d", segCount, tc.wantSegCount)
			}

			if !tc.wantErr {
				// Check that the returned command starts with the expected prefix (if any)
				if tc.wantCmdPrefix != "" && !strings.HasPrefix(cmd, tc.wantCmdPrefix) {
					t.Errorf("parseNextResp() cmd prefix mismatch, got: %q, want prefix: %q", cmd, tc.wantCmdPrefix)
				}
				// Leftover should be what's left after reading one command
				if leftover != "" && leftover == tc.input {
					t.Errorf("parseNextResp() leftover not consumed, leftover=%q", leftover)
				}
			}
		})
	}
}

// TestParsePipelineWithTotalSegs tests the parsePipelineWithTotalSegs function,
// which parses multiple RESP commands in pipeline and checks if the sum of
// all command segments equals a specified totalSegs.
func TestParsePipelineWithTotalSegs(t *testing.T) {
	// Construct a pipeline of three commands:
	// Cmd1: *2\r\n$3\r\nGET\r\n$3\r\nkey\r\n -> 2 segments
	// Cmd2: *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n -> 3 segments
	// Cmd3: *2\r\n$3\r\nGET\r\n$3\r\nkey\r\n -> 2 segments
	// Total = 7 segments
	pipeline := "" +
		"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n" +
		"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n" +
		"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n" +
		"Some leftover data"

	// A pipeline that extends the above with one more command (2 segments).
	// So total = 9 segments now.
	extendedPipeline := pipeline + "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
	truncatedPipeline := "" +
		"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n" +
		"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n" +
		"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n" +
		"*2\r" // "*2\r" is a truncated stmt

	tests := []struct {
		name      string
		input     string
		totalSegs int
		wantErr   bool
		equalTo   string
	}{
		{
			name:      "Exact match: totalSegs=5 on pipeline of 7 segments",
			input:     pipeline,
			totalSegs: 5,
			wantErr:   false,
			equalTo:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		},
		{
			name:      "Exact match: totalSegs=7 on pipeline of 7 segments",
			input:     pipeline,
			totalSegs: 7,
			wantErr:   false,
			equalTo:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			name:      "Exact match: totalSegs=8 on pipeline of 7 segments",
			input:     pipeline + "END",
			totalSegs: 8,
			wantErr:   true,
			equalTo:   "",
		},
		{
			name:      "Less segs than needed: totalSegs=6 on pipeline of 7",
			input:     pipeline,
			totalSegs: 6,
			wantErr:   true,
			equalTo:   "",
		},
		{
			name:      "Extended pipeline total=7, but we only ask for the first 7 segs",
			input:     extendedPipeline,
			totalSegs: 7,
			wantErr:   false,
			equalTo:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			name:      "Truncated pipeline total=7, but we only ask for the first 8 segs",
			input:     truncatedPipeline,
			totalSegs: 8,
			wantErr:   false,
			equalTo:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			name:      "Extended pipeline total=7, but we only ask for 8 segs",
			input:     extendedPipeline,
			totalSegs: 8,
			wantErr:   true,
			equalTo:   "",
		},
		{
			name:      "Extended pipeline total=7, ask for 9 => success",
			input:     extendedPipeline,
			totalSegs: 9,
			wantErr:   true,
			equalTo:   "",
		},
		{
			name:      "Zero totalSegs => error",
			input:     pipeline,
			totalSegs: 0,
			wantErr:   false,
			equalTo:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			name:      "Negative totalSegs => error",
			input:     pipeline,
			totalSegs: -1,
			wantErr:   true,
			equalTo:   "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePipelineWithTotalSegs(tc.totalSegs, tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parsePipelineWithTotalSegs() error = %v, wantErr = %v",
					err, tc.wantErr)
				return
			}
			if !tc.wantErr && tc.equalTo != "" && got != tc.equalTo {
				t.Errorf("parsePipelineWithTotalSegs() output does not contain %q\nOutput:\n%q",
					tc.equalTo, got)
			}
		})
	}
}
