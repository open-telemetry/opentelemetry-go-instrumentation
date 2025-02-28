// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

var testGoVersion = semver.New(1, 22, 1, "", "")

type TestProbe interface {
	Spec() (*ebpf.CollectionSpec, error)
	InjectConsts(*process.Info, *ebpf.CollectionSpec) error
}

func ProbesLoad(t *testing.T, p TestProbe, libs map[string]*semver.Version) {
	err := rlimit.RemoveMemlock()
	if !assert.NoError(t, err) {
		return
	}

	info := &process.Info{
		PID: 1,
		Allocation: &process.Allocation{
			StartAddr: 140434497441792,
			EndAddr:   140434497507328,
		},
		Modules: map[string]*semver.Version{
			"std": testGoVersion,
		},
		GoVersion: testGoVersion,
	}
	for k, v := range libs {
		info.Modules[k] = v
	}

	err = bpffs.Mount(info)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		_ = bpffs.Cleanup(info)
	}()

	spec, err := p.Spec()
	if !assert.NoError(t, err) {
		return
	}

	// Inject the same constants as the BPF program.
	// It is important to inject the same constants as those that will be used in the actual run,
	// since From Linux 5.5 the verifier will use constants to eliminate dead code.
	err = p.InjectConsts(info, spec)
	if !assert.NoError(t, err) {
		return
	}

	opts := ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(info),
		},
	}

	collectVerifierLogs := utils.ShouldShowVerifierLogs()
	if collectVerifierLogs {
		opts.Programs.LogLevel = ebpf.LogLevelStats | ebpf.LogLevelInstruction
	}

	c, err := ebpf.NewCollectionWithOptions(spec, opts)
	if !assert.NoError(t, err) {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) && collectVerifierLogs {
			t.Logf("Verifier log: %-100v\n", ve)
		}
	}

	defer func() {
		if c != nil {
			c.Close()
		}
	}()
}
