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
	"strings"
	"syscall"

	"github.com/hashicorp/go-version"
)

// GetLinuxKernelVersion finds the kernel version using 'uname' syscall.
func GetLinuxKernelVersion() (*version.Version, error) {
	var utsname syscall.Utsname

	if err := syscall.Uname(&utsname); err != nil {
		return nil, err
	}

	var buf [65]byte
	for i, v := range utsname.Release {
		buf[i] = byte(v)
	}

	ver := string(buf[:])
	if strings.Contains(ver, "-") {
		ver = strings.Split(ver, "-")[0]
	}

	return version.NewVersion(ver)
}