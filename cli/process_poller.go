// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPollInterval = 2 * time.Second
	procDir             = "/proc"
)

// ProcessPoller polls the process tree for a target process.
type ProcessPoller struct {
	// Logger is used to log updates about the polling.
	Logger *slog.Logger
	// BinPath is the path of the target process executable.
	BinPath string
	// Interval is time between successive polling attempts. If zero, a default
	// of 2 seconds will be used.
	Interval time.Duration
}

func (pp *ProcessPoller) interval() time.Duration {
	if pp.Interval <= 0 {
		return defaultPollInterval
	}
	return pp.Interval
}

var discardLogger = slog.New(discardHandler{})

// Replace with slog.DiscardHandler when Go 1.23 support is dropped.
type discardHandler struct{}

func (dh discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (dh discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (dh discardHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return dh }
func (dh discardHandler) WithGroup(name string) slog.Handler        { return dh }

func (pp *ProcessPoller) logger() *slog.Logger {
	if pp.Logger != nil {
		return pp.Logger
	}
	return discardLogger
}

// Poll polls the processes running on the system. The first process discovered
// that is running with the configured BinPath will have its PID returned.
func (pp *ProcessPoller) Poll(ctx context.Context) (int, error) {
	path, err := filepath.Abs(pp.BinPath)
	if err != nil {
		return 0, err
	}

	interval := pp.interval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pp.logger().Info(
		"Polling for process",
		"binary", path,
		"interval", interval,
	)
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
			pid, err := pp.find(path)
			if err != nil {
				pp.logger().Error("failed to poll processes", "error", err)
				continue
			}

			if pid < 0 {
				pp.logger().Debug(
					"process not found, continuing...",
					"binary", path,
				)
				continue
			}
			pp.logger().Info("process found", "PID", pid)
			return pid, nil
		}
	}
}

// Overwritten in testing.
var (
	osReadDir  = os.ReadDir
	osReadlink = os.Readlink
	osReadFile = os.ReadFile
)

func (pp *ProcessPoller) find(path string) (int, error) {
	entries, err := osReadDir(procDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", procDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		exe, err := osReadlink(procDir + "/" + name + "/exe")
		if err != nil {
			// Readlink may fail if the target process is not run as root.
			cmdLine, err := osReadFile(procDir + "/" + name + "/cmdline")
			if err != nil {
				return 0, err
			}

			if strings.Contains(string(cmdLine), path) {
				return pid, nil
			}
		} else if exe == path {
			return pid, nil
		}
	}
	return -1, nil
}
