// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
)

const (
	showVerifierLogEnvVar = "OTEL_GO_AUTO_SHOW_VERIFIER_LOG"
)

// InitializeEBPFCollection loads eBPF objects from the given spec and returns a collection corresponding to the spec.
// If the environment variable OTEL_GO_AUTO_SHOW_VERIFIER_LOG is set to true, the verifier log will be printed.
func InitializeEBPFCollection(
	spec *ebpf.CollectionSpec,
	opts *ebpf.CollectionOptions,
) (*ebpf.Collection, error) {
	// Getting full verifier log is expensive, so we only do it if the user explicitly asks for it.
	showVerifierLogs := ShouldShowVerifierLogs()
	if showVerifierLogs {
		opts.Programs.LogLevel = ebpf.LogLevelInstruction | ebpf.LogLevelBranch | ebpf.LogLevelStats
	}

	c, err := ebpf.NewCollectionWithOptions(spec, *opts)
	if err != nil && showVerifierLogs {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			fmt.Printf("Verifier log: %-100v\n", ve)
		}
	}

	return c, err
}

// ShouldShowVerifierLogs returns if the user has configured verifier logs to be emitted.
func ShouldShowVerifierLogs() bool {
	val, exists := os.LookupEnv(showVerifierLogEnvVar)
	if exists {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}

// SupportsContextPropagation returns if the Linux kernel supports use of
// bpf_probe_write_user. It will check for supported versions of the Linux
// kernel and then verify if /sys/kernel/security/lockdown is not locked down.
func SupportsContextPropagation() bool {
	ver := GetLinuxKernelVersion()
	if ver == nil {
		return false
	}

	noLockKernel := semver.New(5, 14, 0, "", "")
	if ver.LessThan(noLockKernel) {
		return true
	}

	return KernelLockdownMode() == KernelLockdownNone
}
