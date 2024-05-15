package log_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/auto/internal/pkg/log"
)

func TestLevel(t *testing.T) {
	testCases := []struct {
		name  string
		level log.Level
		str   string
	}{
		{
			name:  "LevelDebug",
			level: log.LevelDebug,
			str:   "debug",
		},
		{
			name:  "LevelInfo",
			level: log.LevelInfo,
			str:   "info",
		},
		{
			name:  "LevelWarn",
			level: log.LevelWarn,
			str:   "warn",
		},
		{
			name:  "LevelError",
			level: log.LevelError,
			str:   "error",
		},
		{
			name:  "LevelDPanic",
			level: log.LevelDPanic,
			str:   "dpanic",
		},
		{
			name:  "LevelPanic",
			level: log.LevelPanic,
			str:   "panic",
		},
		{
			name:  "LevelFatal",
			level: log.LevelFatal,
			str:   "fatal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.str, tc.level.String(), "string does not match")
		})
	}
}
