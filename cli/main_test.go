// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	appPath        = "/home/fake/bin/app"
	appPathPID int = 1000

	altPath        = "/home/fake/bin/alt"
	altPathPID int = 1001

	missingPath = "/home/fake/bin/missing"
)

var pids = map[string]int{appPath: appPathPID, altPath: altPathPID}

var errMissing = errors.New("missing")

func fakeFindExe(_ context.Context, _ *slog.Logger, exe string) (int, error) {
	pid, ok := pids[exe]
	if !ok {
		return -1, errMissing
	}
	return pid, nil
}

func TestFindPID(t *testing.T) {
	orig := findExeFn
	findExeFn = fakeFindExe
	t.Cleanup(func() { findExeFn = orig })

	ctx := context.Background()

	t.Run("Empty", func(t *testing.T) {
		got, err := findPID(ctx, discardLogger, -1, "")
		assert.Equal(t, -1, got)
		assert.ErrorIs(t, err, errNoPID)
	})

	t.Run("PID", func(t *testing.T) {
		const pid = 4
		got, err := findPID(ctx, discardLogger, pid, appPath)
		assert.NoError(t, err)
		assert.Equal(t, int(pid), got)
	})

	t.Run("BinPath", func(t *testing.T) {
		got, err := findPID(ctx, discardLogger, -1, appPath)
		assert.NoError(t, err)
		assert.Equal(t, appPathPID, got)

		got, err = findPID(ctx, discardLogger, -1, missingPath)
		assert.Equal(t, -1, got)
		assert.ErrorIs(t, err, errMissing)
	})

	t.Run("OTEL_GO_AUTO_TARGET_PID", func(t *testing.T) {
		t.Setenv(envTargetPIDKey, "2000")
		got, err := findPID(ctx, discardLogger, -1, "")
		assert.NoError(t, err)
		assert.Equal(t, int(2000), got)

		t.Setenv(envTargetPIDKey, "invalid")
		_, err = findPID(ctx, discardLogger, -1, "")
		assert.Error(t, err)
	})

	t.Run("OTEL_GO_AUTO_TARGET_EXE", func(t *testing.T) {
		t.Setenv(envTargetExeKey, appPath)
		got, err := findPID(ctx, discardLogger, -1, "")
		assert.NoError(t, err)
		assert.Equal(t, appPathPID, got)
	})

	t.Run("Precedence", func(t *testing.T) {
		t.Setenv(envTargetPIDKey, "2000")
		t.Setenv(envTargetExeKey, altPath)

		const pid = 4
		got, err := findPID(ctx, discardLogger, pid, appPath)
		assert.NoError(t, err)
		assert.Equal(t, int(pid), got)

		got, err = findPID(ctx, discardLogger, -1, appPath)
		assert.NoError(t, err)
		assert.Equal(t, appPathPID, got)

		got, err = findPID(ctx, discardLogger, -1, "")
		assert.NoError(t, err)
		assert.Equal(t, 2000, got)

		os.Unsetenv(envTargetPIDKey)

		got, err = findPID(ctx, discardLogger, -1, "")
		assert.NoError(t, err)
		assert.Equal(t, altPathPID, got)
	})
}
