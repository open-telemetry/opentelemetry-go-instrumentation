// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setContent(t *testing.T, path, text string) {
	err := os.WriteFile(path, []byte(text), 0o600)
	assert.NoError(t, err)
}

func setNotReadable(t *testing.T, path string) {
	err := os.Chmod(path, 0o00)
	assert.NoError(t, err)
}

func TestGetCPUCountFromSysDevices(t *testing.T) {
	noFile, err := os.CreateTemp(t.TempDir(), "not_existent_fake_cpu_present")
	assert.NoError(t, err)
	notPath, err := filepath.Abs(noFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, noFile.Close())
	assert.NoError(t, os.Remove(noFile.Name()))

	// Setup for testing file that doesn't exist
	cpuPresentPath = notPath
	ncpu, err := getCPUCountFromSysDevices()
	assert.Error(t, err)
	assert.Equal(t, uint64(0), ncpu)

	tempFile, err := os.CreateTemp(t.TempDir(), "fake_cpu_present")
	assert.NoError(t, err)
	path, err := filepath.Abs(tempFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, tempFile.Close())

	defer os.Remove(tempFile.Name())
	// Setup for testing
	cpuPresentPath = path

	setContent(t, path, "0-7")
	ncpu, err = getCPUCountFromSysDevices()
	assert.NoError(t, err)
	assert.Equal(t, uint64(8), ncpu)

	setContent(t, path, "0-7,10-15")
	ncpu, err = getCPUCountFromSysDevices()
	assert.NoError(t, err)
	assert.Equal(t, uint64(14), ncpu)

	setContent(t, path, "0-7,10-15,20-23")
	ncpu, err = getCPUCountFromSysDevices()
	assert.NoError(t, err)
	assert.Equal(t, uint64(18), ncpu)

	setContent(t, path, "0-")
	ncpu, err = getCPUCountFromSysDevices()
	assert.Error(t, err)
	assert.Equal(t, uint64(0), ncpu)

	setNotReadable(t, path)
	ncpu, err = getCPUCountFromSysDevices()
	assert.Error(t, err)
	assert.Equal(t, uint64(0), ncpu)
}

func TestGetCPUCountFromProc(t *testing.T) {
	noFile, err := os.CreateTemp(t.TempDir(), "not_existent_fake_cpuinfo")
	assert.NoError(t, err)
	notPath, err := filepath.Abs(noFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, noFile.Close())
	assert.NoError(t, os.Remove(noFile.Name()))

	// Setup for testing file that doesn't exist
	procInfoPath = notPath
	ncpu, err := getCPUCountFromProc()
	assert.Error(t, err)
	assert.Equal(t, uint64(0), ncpu)

	tempFile, err := os.CreateTemp(t.TempDir(), "fake_cpuinfo")
	assert.NoError(t, err)
	path, err := filepath.Abs(tempFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, tempFile.Close())

	defer os.Remove(tempFile.Name())
	// Setup for testing
	procInfoPath = path

	setContent(t, path, "processor	: 0")
	ncpu, err = getCPUCountFromProc()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), ncpu)

	setContent(t, path, "processor	: 0\nprocessor	: 1")
	ncpu, err = getCPUCountFromProc()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), ncpu)

	setContent(t, path, "processor	: 0\nprocessor	: 1\nprocessor	: 2")
	ncpu, err = getCPUCountFromProc()
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), ncpu)

	setContent(t, path, "processor	: 0\nprocessor	: 1\nprocessor	: 2\nprocessor	: 3")
	ncpu, err = getCPUCountFromProc()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), ncpu)

	setContent(t, path, "processor	: 0\n some text \nprocessor	: 1")
	ncpu, err = getCPUCountFromProc()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), ncpu)

	setNotReadable(t, path)
	ncpu, err = getCPUCountFromProc()
	assert.Error(t, err)
	assert.Equal(t, uint64(0), ncpu)
}
