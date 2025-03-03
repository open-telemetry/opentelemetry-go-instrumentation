// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/buildinfo"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setup sets up a temporary testing process directory:
//
//	{temp}/
//	{temp}/app
//	{temp}/{pid}
//	{temp}/task/
//	{temp}/task/{task_0 ...}
//
// The testing app file handle is returned.
//
// All created files and directories are cleaned up on test exit.
func setup(t *testing.T, pid ID, tasks ...string) *os.File {
	t.Helper()

	// Override procDir for testing.
	orig := procDir
	t.Cleanup(func() { procDir = orig })

	tmpDir := t.TempDir()
	t.Logf("temp dir: %s", tmpDir)
	procDir = func(id ID) string {
		return filepath.Join(tmpDir, strconv.Itoa(int(id)))
	}

	// Add a proc entry for pid.
	require.NoError(t, os.MkdirAll(procDir(pid), 0o755))

	// Create directories for any tasks.
	taskPath := filepath.Join(procDir(pid), "task")
	for _, task := range tasks {
		path := filepath.Join(taskPath, task)
		require.NoError(t, os.MkdirAll(path, 0o755))
	}

	// Create the app. Do not link, leave that for the tests.
	app, err := os.Create(filepath.Join(tmpDir, "app"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	// Used for debugging test failures (is not prited on success).
	_ = filepath.Walk(tmpDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		t.Log(path)
		return nil
	})

	return app
}

func TestIDValidate(t *testing.T) {
	t.Run("InvalidID", func(t *testing.T) {
		err := ID(-1).Validate()
		assert.ErrorIs(t, err, errInvalidID)
	})

	t.Run("NoProcess", func(t *testing.T) {
		orig := osFindProcess
		t.Cleanup(func() { osFindProcess = orig })
		osFindProcess = func(int) (*os.Process, error) {
			return nil, assert.AnError
		}

		err := ID(1).Validate()
		assert.ErrorIs(t, err, errNoID)
		assert.ErrorIs(t, err, assert.AnError, "original error dropped")
	})

	t.Run("NoRunningProcess", func(t *testing.T) {
		t.Cleanup(func(orig func(int) (*os.Process, error)) func() {
			p := new(os.Process)
			osFindProcess = func(int) (*os.Process, error) { return p, nil }
			return func() { osFindProcess = orig }
		}(osFindProcess))

		t.Cleanup(func(orig func(p *os.Process, s os.Signal) error) func() {
			sig = func(p *os.Process, s os.Signal) error {
				return assert.AnError
			}
			return func() { sig = orig }
		}(sig))

		err := ID(1).Validate()
		assert.ErrorIs(t, err, errNoRunID)
		assert.ErrorIs(t, err, assert.AnError, "original error dropped")
	})

	t.Run("NoError", func(t *testing.T) {
		t.Cleanup(func(orig func(int) (*os.Process, error)) func() {
			p := new(os.Process)
			osFindProcess = func(int) (*os.Process, error) { return p, nil }
			return func() { osFindProcess = orig }
		}(osFindProcess))

		t.Cleanup(func(orig func(p *os.Process, s os.Signal) error) func() {
			sig = func(p *os.Process, s os.Signal) error { return nil }
			return func() { sig = orig }
		}(sig))

		assert.NoError(t, ID(1).Validate())
	})
}

func TestIDExeLink(t *testing.T) {
	const pid = 100
	app := setup(t, pid)

	ln := filepath.Join(procDir(pid), "exe")

	t.Run("AbsolutePath", func(t *testing.T) {
		require.NoError(t, os.Symlink(app.Name(), ln))
		t.Cleanup(func() { require.NoError(t, os.Remove(ln)) })

		got, err := ID(pid).ExeLink()
		require.NoError(t, err)
		assert.Equal(t, app.Name(), got)
	})

	t.Run("RelativePath", func(t *testing.T) {
		require.NoError(t, os.Symlink("../app", ln))
		t.Cleanup(func() { require.NoError(t, os.Remove(ln)) })

		got, err := ID(pid).ExeLink()
		require.NoError(t, err)
		assert.Equal(t, app.Name(), got)
	})
}

func TestIDTasks(t *testing.T) {
	const pid = 100
	dirs := []string{"1234", "4321"}
	_ = setup(t, pid, dirs...)

	entries, err := ID(pid).Tasks()
	require.NoError(t, err)

	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	assert.Equal(t, dirs, got)
}

func TestIDBuildInfo(t *testing.T) {
	const pid = 100
	app := setup(t, pid)

	ln := filepath.Join(procDir(pid), "exe")
	require.NoError(t, os.Symlink(app.Name(), ln))

	orig := buildinfoReadFile
	t.Cleanup(func() { buildinfoReadFile = orig })

	buildinfoReadFile = func(name string) (*buildinfo.BuildInfo, error) {
		assert.Equal(t, ID(pid).ExePath(), name, "wrong exe path")
		return &debug.BuildInfo{
			Path:      app.Name(),
			GoVersion: "go1.22.0 X:testing",
		}, nil
	}

	got, err := ID(pid).BuildInfo()
	require.NoError(t, err)

	want := &debug.BuildInfo{Path: app.Name(), GoVersion: "1.22.0"}
	assert.Equal(t, want, got)
}
