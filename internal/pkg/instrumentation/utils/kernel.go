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
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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

type KernelLockdown uint8

const (
	KernelLockdownNone            KernelLockdown = iota + 1 // Linux Kernel security lockdown mode [none]
	KernelLockdownIntegrity                                 // Linux Kernel security lockdown mode [integrity]
	KernelLockdownConfidentiality                           // Linux Kernel security lockdown mode [confidentiality]
	KernelLockdownOther                                     // Linux Kernel security lockdown mode unknown
)

// Injectable for tests.
var lockdownPath = "/sys/kernel/security/lockdown"

func KernelLockdownMode() KernelLockdown {
	// If we can't find the file, assume no lockdown
	if _, err := os.Stat(lockdownPath); err == nil {
		f, err := os.Open(lockdownPath)
		if err != nil {
			return KernelLockdownIntegrity
		}

		defer f.Close()
		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			lockdown := scanner.Text()
			if strings.Contains(lockdown, "[none]") {
				return KernelLockdownNone
			} else if strings.Contains(lockdown, "[integrity]") {
				return KernelLockdownIntegrity
			} else if strings.Contains(lockdown, "[confidentiality]") {
				return KernelLockdownConfidentiality
			}
			return KernelLockdownOther
		}

		return KernelLockdownIntegrity
	}

	return KernelLockdownNone
}

// Injectable for tests, this file is one of the files LSCPU looks for.
var cpuPresentPath = "/sys/devices/system/cpu/present"

func GetCPUCountFromSysDevices() (int, error) {
	rawFile, err := os.ReadFile(cpuPresentPath)
	if err != nil {
		return 0, err
	}

	cpuCount, err := parseCPUList(string(rawFile))
	if err != nil {
		return 0, err
	}

	return cpuCount, nil
}

func parseCPUList(raw string) (int, error) {
	listPart := strings.Split(raw, ",")
	count := 0
	for _, v := range listPart {
		if strings.Contains(v, "-") {
			rangeC, err := parseCPURange(v)
			if err != nil {
				return 0, fmt.Errorf("error parsing line %s: %w", v, err)
			}
			count = count + rangeC
		} else {
			count++
		}
	}
	return count, nil
}

func parseCPURange(cpuRange string) (int, error) {
	var first, last int
	_, err := fmt.Sscanf(cpuRange, "%d-%d", &first, &last)
	if err != nil {
		return 0, fmt.Errorf("error reading from range %s: %w", cpuRange, err)
	}

	return (last - first) + 1, nil
}

var procInfoPath = "/proc/cpuinfo"

func GetCPUCountFromProc() (int, error) {
	file, err := os.Open(procInfoPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "processor") {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

func GetCPUCount() (int, error) {
	var err error
	// First try to get the CPU count from /sys/devices
	cpuCount, e := GetCPUCountFromSysDevices()
	if e == nil {
		return cpuCount, nil
	}
	err = errors.Join(err, e)

	// If that fails, try to get the CPU count from /proc
	cpuCount, e = GetCPUCountFromProc()
	if e == nil {
		return cpuCount, nil
	}
	err = errors.Join(err, e)

	return 0, err
}
