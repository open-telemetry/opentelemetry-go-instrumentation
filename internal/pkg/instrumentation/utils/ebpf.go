// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/cilium/ebpf"
	"github.com/hashicorp/go-version"
)

const (
	showVerifierLogEnvVar = "OTEL_GO_AUTO_SHOW_VERIFIER_LOG"
)

// InitializeEBPFCollection loads eBPF objects from the given spec and returns a collection corresponding to the spec.
// If the environment variable OTEL_GO_AUTO_SHOW_VERIFIER_LOG is set to true, the verifier log will be printed.
func InitializeEBPFCollection(spec *ebpf.CollectionSpec, opts *ebpf.CollectionOptions) (*ebpf.Collection, error) {
	// Getting full verifier log is expensive, so we only do it if the user explicitly asks for it.
	showVerifierLogs := shouldShowVerifierLogs()
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

// shouldShowVerifierLogs returns if the user has configured verifier logs to be emitted.
func shouldShowVerifierLogs() bool {
	val, exists := os.LookupEnv(showVerifierLogEnvVar)
	if exists {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}

// Does kernel version check and /sys/kernel/security/lockdown inspection to determine if it's
// safe to use bpf_probe_write_user.
func SupportsContextPropagation() bool {
	ver, err := GetLinuxKernelVersion()
	if err != nil {
		return false
	}

	noLockKernel, err := version.NewVersion("5.14")
	if err != nil {
		fmt.Printf("Error creating version 5.14 - %v\n", err)
	}

	if ver.LessThan(noLockKernel) {
		return true
	}

	return KernelLockdownMode() == KernelLockdownNone
}
