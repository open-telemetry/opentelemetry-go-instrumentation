// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package kernel

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Injectable for tests, this file is one of the files LSCPU looks for.
var cpuPresentPath = "/sys/devices/system/cpu/present"

func getCPUCountFromSysDevices() (uint64, error) {
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

func getCPUCountFromProc() (uint64, error) {
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

func cpuCount() (uint64, error) {
	var err error
	// First try to get the CPU count from /sys/devices
	cpuCount, e := getCPUCountFromSysDevices()
	if e == nil {
		return cpuCount, nil
	}
	err = errors.Join(err, e)

	// If that fails, try to get the CPU count from /proc
	cpuCount, e = getCPUCountFromProc()
	if e == nil {
		return cpuCount, nil
	}
	err = errors.Join(err, e)

	return 0, err
}
