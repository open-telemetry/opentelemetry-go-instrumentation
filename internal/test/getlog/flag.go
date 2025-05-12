// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package getlog provides logger setup utilities for end-to-end integration
// testing applications.
package getlog

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

// envLogLevelKey is the key for the environment variable value containing the
// log level.
const envLogLevelKey = "OTEL_LOG_LEVEL"

// Flag is a [flag.Value] that parses and handles a user provided log level
// argument.
//
// Its common use case looks like this:
//
//	var f getlog.Flag
//	flag.Var(&f, "log-level", f.Docs())
//	flag.Parse()
//	logger := f.Logger()
type Flag struct {
	value string
	l     *slog.Logger
}

var _ flag.Value = (*Flag)(nil)

// Docs returns documentation about the Level for the command line.
func (l *Flag) Docs() string {
	return `Logging level ("debug", "info", "warn", "error") [ENV: ` + envLogLevelKey + `]`
}

// Set sets the logger of the Flag.
func (l *Flag) Set(s string) error { return l.set(s) }

func (l *Flag) set(lvlStr string) error {
	levelVar := new(slog.LevelVar) // Default value of info.
	opts := &slog.HandlerOptions{AddSource: true, Level: levelVar}
	h := slog.NewTextHandler(os.Stderr, opts)
	l.l = slog.New(h)

	if lvlStr == "" {
		lvlStr = os.Getenv(envLogLevelKey)
	}

	if lvlStr == "" {
		lvlStr = "info"
	}

	l.value = lvlStr

	var level slog.Level
	err := level.UnmarshalText([]byte(lvlStr))
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}
	levelVar.Set(level)

	return nil
}

// String returns the configured logger level value.
func (l *Flag) String() string {
	if l == nil {
		return ""
	}
	return l.value
}

// Logger returns the configured logger. If the logger is not set, it
// initializes it with the default value.
func (l *Flag) Logger() *slog.Logger {
	if l == nil {
		return slog.Default()
	}

	if l.l == nil {
		if err := l.set(""); err != nil {
			return slog.Default()
		}
	}

	return l.l
}
