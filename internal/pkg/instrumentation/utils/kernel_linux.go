// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Masterminds/semver/v3"
)

var unameFn = syscall.Uname

// GetLinuxKernelVersion returns the current version of the Linux kernel. If
// unable to determine the function, nil is returned.
//
// Adapted from https://github.com/golang/go/blob/go1.21.3/src/internal/syscall/unix/kernel_version_linux.go
func GetLinuxKernelVersion() *semver.Version {
	var uname syscall.Utsname
	if err := unameFn(&uname); err != nil {
		return nil
	}

	var (
		values [2]uint64
		value  uint64
		vi     int
	)
	for _, c := range uname.Release {
		if '0' <= c && c <= '9' {
			value = (value * 10) + uint64(c-'0') // nolint:gosec  // c >= '0'
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
	return semver.New(values[0], values[1], 0, "", "")
}

// KernelLockdown is the lockdown state of the Linux kernel.
type KernelLockdown uint8

const (
	// KernelLockdownNone is the "none" Linux Kernel security lockdown mode.
	KernelLockdownNone KernelLockdown = iota + 1
	// KernelLockdownIntegrity is the "integrity" Linux Kernel security lockdown mode.
	KernelLockdownIntegrity
	// KernelLockdownConfidentiality is the "confidentiality" Linux Kernel security lockdown mode.
	KernelLockdownConfidentiality
	// KernelLockdownOther is the "unknown" Linux Kernel security lockdown mode.
	KernelLockdownOther
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

func GetCPUCountFromSysDevices() (uint64, error) {
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

func parseCPUList(raw string) (uint64, error) {
	listPart := strings.Split(raw, ",")
	var count uint64
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

func parseCPURange(cpuRange string) (uint64, error) {
	var first, last uint64
	_, err := fmt.Sscanf(cpuRange, "%d-%d", &first, &last)
	if err != nil {
		return 0, fmt.Errorf("error reading from range %s: %w", cpuRange, err)
	}

	return (last - first) + 1, nil
}

var procInfoPath = "/proc/cpuinfo"

func GetCPUCountFromProc() (uint64, error) {
	file, err := os.Open(procInfoPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var count uint64
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

func GetCPUCount() (uint64, error) {
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

func estimateBootTimeOffset() (bootTimeOffset int64, err error) {
	// The datapath is currently using ktime_get_boot_ns for the pcap timestamp,
	// which corresponds to CLOCK_BOOTTIME. To be able to convert the the
	// CLOCK_BOOTTIME to CLOCK_REALTIME (i.e. a unix timestamp).

	// There can be an arbitrary amount of time between the execution of
	// time.Now() and unix.ClockGettime() below, especially under scheduler
	// pressure during program startup. To reduce the error introduced by these
	// delays, we pin the current Go routine to its OS thread and measure the
	// clocks multiple times, taking only the smallest observed difference
	// between the two values (which implies the smallest possible delay
	// between the two snapshots).
	var minDiff int64 = 1<<63 - 1
	estimationRounds := 25
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for round := 0; round < estimationRounds; round++ {
		var bootTimespec unix.Timespec

		// Ideally we would use __vdso_clock_gettime for both clocks here,
		// to have as little overhead as possible.
		// time.Now() will actually use VDSO on Go 1.9+, but calling
		// unix.ClockGettime to obtain CLOCK_BOOTTIME is a regular system call
		// for now.
		unixTime := time.Now()
		err = unix.ClockGettime(unix.CLOCK_BOOTTIME, &bootTimespec)
		if err != nil {
			return 0, err
		}

		offset := unixTime.UnixNano() - bootTimespec.Nano()
		diff := offset
		if diff < 0 {
			diff = -diff
		}

		if diff < minDiff {
			minDiff = diff
			bootTimeOffset = offset
		}
	}

	return bootTimeOffset, nil
}
