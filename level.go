// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"bytes"
	"errors"
	"fmt"
)

// LogLevel defines the log level which instrumentation uses.
type LogLevel string

const (
	// logLevelUndefined is an unset log level, it should not be used.
	logLevelUndefined LogLevel = ""
	// LogLevelDebug sets the logging level to log all messages.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo sets the logging level to log only informational, warning, and error messages.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn sets the logging level to log only warning and error messages.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError sets the logging level to log only error messages.
	LogLevelError LogLevel = "error"
)

var errInvalidLogLevel = errors.New("invalid LogLevel")

// String returns the string encoding of the LogLevel l.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, logLevelUndefined:
		return string(l)
	default:
		return fmt.Sprintf("Level(%s)", string(l))
	}
}

// UnmarshalText applies the LogLevel type when inputted text is valid.
func (l *LogLevel) UnmarshalText(text []byte) error {
	*l = LogLevel(bytes.ToLower(text))

	return l.validate()
}

func (l *LogLevel) validate() error {
	if l == nil {
		return errors.New("nil LogLevel")
	}

	switch *l {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		// Valid.
	default:
		return fmt.Errorf("%w: %s", errInvalidLogLevel, l.String())
	}
	return nil
}

// ParseLogLevel return a new LogLevel parsed from text. A non-nil error is returned if text is not a valid LogLevel.
func ParseLogLevel(text string) (LogLevel, error) {
	var level LogLevel

	err := level.UnmarshalText([]byte(text))

	return level, err
}
