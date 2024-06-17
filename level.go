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

package auto

import (
	"bytes"
	"errors"
	"fmt"
)

// Level defines the log level which instrumentation uses.
type LogLevel string

const (
	// LevelUndefined is an unset log level, it should not be used.
	LevelUndefined LogLevel = ""
	// LevelDebug sets the logging level to log all messages.
	LevelDebug LogLevel = "debug"
	// LevelInfo sets the logging level to log only informational, warning, and error messages.
	LevelInfo LogLevel = "info"
	// LevelWarn sets the logging level to log only warning and error messages.
	LevelWarn LogLevel = "warn"
	// LevelError sets the logging level to log only error messages.
	LevelError LogLevel = "error"
)

// String returns the string encoding of the Level l.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug, LevelInfo, LevelWarn, LevelError, LevelUndefined:
		return string(l)
	default:
		return fmt.Sprintf("Level(%s)", string(l))
	}
}

// UnmarshalText applies the LogLevel type when inputted text is valid.
func (l *LogLevel) UnmarshalText(text []byte) error {
	if ok := l.unmarshalText(bytes.ToLower(text)); ok {
		return nil
	}

	return l.validate()
}

func (l *LogLevel) validate() error {
	if l == nil {
		return errors.New("can't parse nil values")
	}

	if !l.unmarshalText(bytes.ToLower([]byte(l.String()))) {
		return errors.New("log level value is not accepted")
	}

	return nil
}

func (l *LogLevel) unmarshalText(text []byte) bool {
	switch string(text) {
	case "debug":
		*l = LevelDebug
	case "info":
		*l = LevelInfo
	case "warn":
		*l = LevelWarn
	case "error":
		*l = LevelError
	default:
		return false
	}

	return true
}

// ParseLevel return a new LogLevel using text, and will return err if inputted text is not accepted.
func ParseLevel(text string) (LogLevel, error) {
	var level LogLevel

	err := level.UnmarshalText([]byte(text))

	return level, err
}
