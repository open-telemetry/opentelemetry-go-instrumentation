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

package log

import (
	"bytes"
	"errors"
	"fmt"
)

type Level string

const (
	// Log Level [debug].
	LevelDebug Level = "debug"
	// Log Level [info].
	LevelInfo Level = "info"
	// Log Level [warn].
	LevelWarn Level = "warn"
	// Log Level [error].
	LevelError Level = "error"
	// Log Level [dpanic].
	LevelDPanic Level = "dpanic"
	// Log Level [panic].
	LevelPanic Level = "panic"
	// Log Level [fatal].
	LevelFatal Level = "fatal"
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelDPanic:
		return "dpanic"
	case LevelPanic:
		return "panic"
	case LevelFatal:
		return "fatal"
	default:
		return fmt.Sprintf("Level(%s)", string(l))
	}
}

func (l *Level) UnmarshalText(text []byte) error {
	if l == nil {
		return errors.New("can't unmarshal nil values")
	}

	if !l.unmarshalText(bytes.ToLower(text)) {
		return fmt.Errorf("")
	}

	return nil
}

func (l *Level) unmarshalText(text []byte) bool {
	switch string(text) {
	case "debug":
		*l = LevelDebug
	case "info":
		*l = LevelInfo
	case "warn":
		*l = LevelWarn
	case "error":
		*l = LevelError
	case "dpanic":
		*l = LevelDPanic
	case "panic":
		*l = LevelPanic
	case "fatal":
		*l = LevelFatal
	default:
		return false
	}

	return true
}

func ParseLevel(text string) (Level, error) {
	var level Level

	err := level.UnmarshalText([]byte(text))

	return level, err
}
