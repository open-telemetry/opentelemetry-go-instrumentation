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
