// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ebpf_test

package instrumentation

import (
	"log/slog"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/testutils"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

func TestLoadProbes(t *testing.T) {
	ver, _ := utils.GetLinuxKernelVersion()
	t.Logf("Running on kernel %s", ver.String())
	m := fakeManager(t)

	probes := m.availableProbes()
	assert.NotEmpty(t, probes)

	for _, p := range probes {
		manifest := p.Manifest()
		fields := manifest.StructFields
		offsets := map[string]*version.Version{}
		for _, f := range fields {
			_, ver := inject.GetLatestOffset(f)
			if ver != nil {
				offsets[f.PkgPath] = ver
				offsets[f.ModPath] = ver
			}
		}
		t.Run(p.Manifest().ID.String(), func(t *testing.T) {
			testProbe, ok := p.(testutils.TestProbe)
			assert.True(t, ok)
			testutils.ProbesLoad(t, testProbe, offsets)
		})
	}
}

func fakeManager(t *testing.T) *Manager {
	m, err := NewManager(slog.Default(), nil, true, NewNoopConfigProvider(nil), "")
	assert.NoError(t, err)
	assert.NotNil(t, m)

	return m
}
