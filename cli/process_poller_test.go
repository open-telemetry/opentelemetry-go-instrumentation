// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type entry struct {
	name   string
	isFile bool
}

func (e entry) Name() string               { return e.name }
func (e entry) IsDir() bool                { return !e.isFile }
func (e entry) Type() fs.FileMode          { return fs.ModeDir }
func (e entry) Info() (fs.FileInfo, error) { return nil, nil }

var errLink = errors.New("simulated link error")

func mock() func() {
	origOSReadDir := osReadDir
	origOSReadlink := osReadlink
	origOSReadFile := osReadFile

	appPIDStr := strconv.Itoa(appPathPID)
	altPIDStr := strconv.Itoa(altPathPID)
	osReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			entry{name: "0"},
			entry{name: "not_a_num"},
			entry{name: "100"},
			entry{name: "101", isFile: true},
			entry{name: appPIDStr},
			entry{name: altPIDStr},
			entry{name: "9000"},
		}, nil
	}

	osReadlink = func(name string) (string, error) {
		switch name {
		case procDir + "/" + appPIDStr + "/exe":
			return appPath, nil
		case procDir + "/" + altPIDStr + "/exe":
			// Test errors are handled
			return "", errLink
		}
		return missingPath, nil
	}

	osReadFile = func(name string) ([]byte, error) {
		switch name {
		case procDir + "/" + appPIDStr + "/cmdline":
			return []byte(appPath + " args"), nil
		case procDir + "/" + altPIDStr + "/cmdline":
			return []byte(altPath + " args"), nil
		}
		return []byte(missingPath + " args"), nil
	}

	return func() {
		osReadDir = origOSReadDir
		osReadlink = origOSReadlink
		osReadFile = origOSReadFile
	}
}

func TestProcessPollerPoll(t *testing.T) {
	t.Cleanup(mock())
	ctx := context.Background()

	pp := ProcessPoller{BinPath: appPath}
	pid, err := pp.Poll(ctx)
	require.NoError(t, err)
	assert.Equal(t, appPathPID, pid)

	pp.BinPath = altPath
	pid, err = pp.Poll(ctx)
	require.NoError(t, err)
	assert.Equal(t, altPathPID, pid)

	pp.Interval = time.Millisecond
	pp.BinPath = "/path/that/is/not/found"
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	pid, err = pp.Poll(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded) //nolint:testifylint // Continue on failure.
	assert.Zero(t, pid)
}
