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

// LoadEBPFObjects loads eBPF objects from the given spec into the given interface.
// If the environment variable OTEL_GO_AUTO_SHOW_VERIFIER_LOG is set to true, the verifier log will be printed.
func LoadEBPFObjects(spec *ebpf.CollectionSpec, to interface{}, opts *ebpf.CollectionOptions) error {
	// Getting full verifier log is expensive, so we only do it if the user explicitly asks for it.
	showVerifierLogs := shouldShowVerifierLogs()
	if showVerifierLogs {
		opts.Programs.LogSize = ebpf.DefaultVerifierLogSize * 100
		opts.Programs.LogLevel = /*ebpf.LogLevelInstruction | ebpf.LogLevelBranch |*/ ebpf.LogLevelStats
	}

	err := spec.LoadAndAssign(to, opts)
	if err != nil && showVerifierLogs {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			fmt.Printf("Verifier log: %-100v\n", ve)
		}
	}

	return err
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
