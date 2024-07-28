// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	testCases := []struct {
		name  string
		level LogLevel
		str   string
	}{
		{
			name:  "LogLevelUndefined",
			level: logLevelUndefined,
			str:   "",
		},
		{
			name:  "LogLevelDebug",
			level: LogLevelDebug,
			str:   "debug",
		},
		{
			name:  "LogLevelInfo",
			level: LogLevelInfo,
			str:   "info",
		},
		{
			name:  "LogLevelWarn",
			level: LogLevelWarn,
			str:   "warn",
		},
		{
			name:  "LogLevelError",
			level: LogLevelError,
			str:   "error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.str, tc.level.String(), "string does not match")
		})
	}
}

func TestValidate(t *testing.T) {
	l := LogLevel("notexist")
	assert.ErrorIs(t, l.validate(), errInvalidLogLevel)
}

func TestParseLogLevel(t *testing.T) {
	testCases := []struct {
		name  string
		str   string
		level LogLevel
	}{
		{
			name:  "ParseLogLevelDebug",
			str:   "debug",
			level: LogLevelDebug,
		},
		{
			name:  "ParseLogLevelInfo",
			str:   "info",
			level: LogLevelInfo,
		},
		{
			name:  "ParseLogLevelWarn",
			str:   "warn",
			level: LogLevelWarn,
		},
		{
			name:  "ParseLogLevelError",
			str:   "error",
			level: LogLevelError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := ParseLogLevel(tc.str)

			assert.Equal(t, tc.level, l, "LogLevel does not match")
		})
	}

	t.Run("ParseNotExist", func(t *testing.T) {
		_, err := ParseLogLevel("notexist")

		assert.ErrorIs(t, err, errInvalidLogLevel)
	})
}
