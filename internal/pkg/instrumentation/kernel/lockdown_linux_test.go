// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLockdownMode(t *testing.T) {
	noFile, err := os.CreateTemp(t.TempDir(), "not_existent_fake_lockdown")
	require.NoError(t, err)
	notPath, err := filepath.Abs(noFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, noFile.Close())
	assert.NoError(t, os.Remove(noFile.Name()))

	// Setup for testing file that doesn't exist
	lockdownPath = notPath
	assert.Equal(t, LockdownModeNone, getLockdownMode())

	tempFile, err := os.CreateTemp(t.TempDir(), "fake_lockdown")
	require.NoError(t, err)
	path, err := filepath.Abs(tempFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, tempFile.Close())

	defer os.Remove(tempFile.Name())
	// Setup for testing
	lockdownPath = path

	setContent(t, path, "none [integrity] confidentiality\n")
	assert.Equal(t, LockdownModeIntegrity, getLockdownMode())

	setContent(t, path, "[none] integrity confidentiality\n")
	assert.Equal(t, LockdownModeNone, getLockdownMode())

	setContent(t, path, "none integrity [confidentiality]\n")
	assert.Equal(t, LockdownModeConfidentiality, getLockdownMode())

	setContent(t, path, "whatever\n")
	assert.Equal(t, LockdownModeOther, getLockdownMode())

	setContent(t, path, "")
	assert.Equal(t, LockdownModeIntegrity, getLockdownMode())

	setContent(t, path, "[none] integrity confidentiality\n")
	setNotReadable(t, path)
	assert.Equal(t, LockdownModeIntegrity, getLockdownMode())
}
