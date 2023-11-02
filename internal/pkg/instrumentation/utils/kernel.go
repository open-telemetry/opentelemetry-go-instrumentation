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
	"fmt"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-version"
)

var unameFn = syscall.Uname

// parse logic adapted from https://github.com/golang/go/blob/go1.21.3/src/internal/syscall/unix/kernel_version_linux.go
func GetLinuxKernelVersion() (*version.Version, error) {
	var uname syscall.Utsname
	if err := unameFn(&uname); err != nil {
		return nil, err
	}

	var (
		values    [2]int
		value, vi int
	)
	for _, c := range uname.Release {
		if '0' <= c && c <= '9' {
			value = (value * 10) + int(c-'0')
		} else {
			// Note that we're assuming N.N.N here.
			// If we see anything else, we are likely to mis-parse it.
			values[vi] = value
			vi++
			if vi >= len(values) {
				break
			}
			value = 0
		}
	}
	ver := fmt.Sprintf("%s.%s", strconv.Itoa(values[0]), strconv.Itoa(values[1]))

	return version.NewVersion(ver)
}
