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

package utils

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func TestGetLinuxKernelVersion(t *testing.T) {
	tests := map[string]struct {
		unameFn func(buf *syscall.Utsname) error
		want    *version.Version
	}{
		"ubuntu-23.10": {
			unameFn: func(buf *syscall.Utsname) error {
				buf.Release = [65]int8{54, 46, 53, 46, 48, 45, 57, 45, 103, 101, 110, 101, 114, 105, 99, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				return nil
			},
			want: version.Must(version.NewVersion("6.5")),
		},
		"debian-12": {
			unameFn: func(buf *syscall.Utsname) error {
				buf.Release = [65]int8{54, 46, 49, 46, 48, 45, 49, 50, 45, 99, 108, 111, 117, 100, 45, 97, 109, 100, 54, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				return nil
			},
			want: version.Must(version.NewVersion("6.1")),
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			oldUnameFn := unameFn
			unameFn = tt.unameFn
			t.Cleanup(func() {
				unameFn = oldUnameFn
			})
			got, err := GetLinuxKernelVersion()
			if err != nil {
				t.Errorf("GetLinuxKernelVersion() error = %v", err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLockdownParsing(t *testing.T) {
	noFile, err := os.CreateTemp("", "not_existent_fake_lockdown")
	assert.NoError(t, err)
	notPath, err := filepath.Abs(noFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, noFile.Close())
	assert.NoError(t, os.Remove(noFile.Name()))

	// Setup for testing file that doesn't exist
	lockdownPath = notPath
	assert.Equal(t, KernelLockdownNone, KernelLockdownMode())

	tempFile, err := os.CreateTemp("", "fake_lockdown")
	assert.NoError(t, err)
	path, err := filepath.Abs(tempFile.Name())
	assert.NoError(t, err)
	assert.NoError(t, tempFile.Close())

	defer os.Remove(tempFile.Name())
	// Setup for testing
	lockdownPath = path

	setIntegrity(t, path, "none [integrity] confidentiality\n")
	assert.Equal(t, KernelLockdownIntegrity, KernelLockdownMode())

	setIntegrity(t, path, "[none] integrity confidentiality\n")
	assert.Equal(t, KernelLockdownNone, KernelLockdownMode())

	setIntegrity(t, path, "none integrity [confidentiality]\n")
	assert.Equal(t, KernelLockdownConfidentiality, KernelLockdownMode())

	setIntegrity(t, path, "whatever\n")
	assert.Equal(t, KernelLockdownOther, KernelLockdownMode())

	setIntegrity(t, path, "")
	assert.Equal(t, KernelLockdownIntegrity, KernelLockdownMode())

	setIntegrity(t, path, "[none] integrity confidentiality\n")
	setNotReadable(t, path)
	assert.Equal(t, KernelLockdownIntegrity, KernelLockdownMode())
}

// Utils.
func setIntegrity(t *testing.T, path, text string) {
	err := os.WriteFile(path, []byte(text), 0o644)
	assert.NoError(t, err)
}

func setNotReadable(t *testing.T, path string) {
	err := os.Chmod(path, 0o00)
	assert.NoError(t, err)
}
