// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package kernel

import (
	"syscall"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	tests := map[string]struct {
		unameFn func(buf *syscall.Utsname) error
		want    *semver.Version
	}{
		"ubuntu-23.10": {
			unameFn: func(buf *syscall.Utsname) error {
				buf.Release = [65]int8{
					54, 46, 53, 46, 48, 45, 57, 45, 103, 101, 110, 101, 114, 105, 99, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				}
				return nil
			},
			want: semver.New(6, 5, 0, "", ""),
		},
		"debian-12": {
			unameFn: func(buf *syscall.Utsname) error {
				buf.Release = [65]int8{
					54, 46, 49, 46, 48, 45, 49, 50, 45, 99, 108, 111, 117, 100, 45, 97, 109,
					100, 54, 52, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				}
				return nil
			},
			want: semver.New(6, 1, 0, "", ""),
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
			got := version()
			require.NotNil(t, got)

			assert.Equal(t, tt.want, got)
		})
	}
}
