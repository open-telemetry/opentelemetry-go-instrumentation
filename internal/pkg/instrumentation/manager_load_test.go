//go:build multi_kernel_test

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"log"
	"os"
	"testing"

	"github.com/go-logr/stdr"
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

	probes := availableProbes(m.logger, true)
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
		t.Run(p.Manifest().Id.String(), func(t *testing.T) {
			testProbe, ok := p.(testutils.TestProbe)
			assert.True(t, ok)
			testutils.ProbesLoad(t, testProbe, offsets)
		})
	}
}

func fakeManager(t *testing.T) *Manager {
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")

	m, err := NewManager(logger, nil, true, nil, NewNoopConfigProvider(nil))
	assert.NoError(t, err)
	assert.NotNil(t, m)

	return m
}
