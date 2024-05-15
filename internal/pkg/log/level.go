package log

import (
	"bytes"
	"errors"
	"fmt"
)

type Level string

const (
	LevelDebug  Level = "debug"
	LevelInfo   Level = "info"
	LevelWarn   Level = "warn"
	LevelError  Level = "error"
	LevelDPanic Level = "dpanic"
	LevelPanic  Level = "panic"
	LevelFatal  Level = "fatal"
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
