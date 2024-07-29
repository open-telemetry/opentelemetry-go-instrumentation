//go:build ebpf_test
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/stretchr/testify/assert"
)

func mockEbpfCollectionForSampling() (*ebpf.Collection, error) {
	err := rlimit.RemoveMemlock()
	if err != nil {
		return nil, err
	}

	samplersConfigMapSpec := &ebpf.MapSpec{
		Name:       samplersConfigMapName,
		Type:       ebpf.Hash,
		KeySize:    4,
		ValueSize:  sampleConfigSize,
		MaxEntries: maxSamplers,
	}

	samplersConfigMap, err := ebpf.NewMapWithOptions(samplersConfigMapSpec, ebpf.MapOptions{})
	if err != nil {
		return nil, err
	}

	activeSamplerMapSpec := &ebpf.MapSpec{
		Name:       probeActiveSamplerMapName,
		Type:       ebpf.Array,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	}

	activeSamplerMap, err := ebpf.NewMapWithOptions(activeSamplerMapSpec, ebpf.MapOptions{})
	if err != nil {
		return nil, err
	}

	return &ebpf.Collection{
		Maps: map[string]*ebpf.Map{
			samplersConfigMapName:     samplersConfigMap,
			probeActiveSamplerMapName: activeSamplerMap,
		},
	}, nil
}

func TestEbpfNewSamplingConfigDefault(t *testing.T) {
	c, err := mockEbpfCollectionForSampling()
	if !assert.NoError(t, err) {
		return
	}

	defer c.Close()
	sc, err := NewSamplingConfig(c)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, sc.currentSamplerID, parentBasedID)

	var activeSamplerIDInMap samplerID
	err = sc.probeActiveSampler.Lookup(uint32(0), &activeSamplerIDInMap)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, activeSamplerIDInMap, parentBasedID)

	var samplersConfigInMap samplerConfig
	err = sc.samplersConfig.Lookup(uint32(parentBasedID), &samplersConfigInMap)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, samplersConfigInMap.samplerType, samplerParentBased)
	assert.Equal(t, samplersConfigInMap.config, defaultParentBasedSampler())
}

func TestEbpfNewSamplingConfigFromEnv(t *testing.T) {
	col, err := mockEbpfCollectionForSampling()
	if !assert.NoError(t, err) {
		return
	}

	defer col.Close()

	cases := []struct {
		testName                  string
		sampler                   string
		samplerArg                string
		expectedSamplerID         samplerID
		expectedSamplerConfig     samplerConfig
		expectedRootSamplerConfig samplerConfig
	}{
		{
			testName:          "parentbased_always_off",
			sampler:           samplerNameParsedBasedAlwaysOff,
			samplerArg:        "",
			expectedSamplerID: parentBasedID,
			expectedSamplerConfig: samplerConfig{
				samplerType: samplerParentBased,
				config: parentBasedConfig{
					Root:             alwaysOffID,
					RemoteSampled:    alwaysOnID,
					RemoteNotSampled: alwaysOffID,
					LocalSampled:     alwaysOnID,
					LocalNotSampled:  alwaysOffID,
				},
			},
		},
		{
			testName:          "parentbased_always_on",
			sampler:           samplerNameParentBasedAlwaysOn,
			samplerArg:        "",
			expectedSamplerID: parentBasedID,
			expectedSamplerConfig: samplerConfig{
				samplerType: samplerParentBased,
				config: parentBasedConfig{
					Root:             alwaysOnID,
					RemoteSampled:    alwaysOnID,
					RemoteNotSampled: alwaysOffID,
					LocalSampled:     alwaysOnID,
					LocalNotSampled:  alwaysOffID,
				},
			},
		},
		{
			testName:          "traceidratio with 0.5",
			sampler:           samplerNameTraceIDRatio,
			samplerArg:        "0.5",
			expectedSamplerID: traceIDRatioID,
			expectedSamplerConfig: samplerConfig{
				samplerType: samplerTraceIDRatio,
				config: traceIDRatioConfig{
					samplingRateNumerator: samplingRateDenominator / 2,
				},
			},
		},
		{
			testName:          "traceidratio with 0.001",
			sampler:           samplerNameTraceIDRatio,
			samplerArg:        "0.001",
			expectedSamplerID: traceIDRatioID,
			expectedSamplerConfig: samplerConfig{
				samplerType: samplerTraceIDRatio,
				config: traceIDRatioConfig{
					samplingRateNumerator: samplingRateDenominator / 1000,
				},
			},
		},
		{
			testName:          "parentbased_traceidratio with 0.1",
			sampler:           samplerNameParentBasedTraceIDRatio,
			samplerArg:        "0.1",
			expectedSamplerID: parentBasedID,
			expectedSamplerConfig: samplerConfig{
				samplerType: samplerParentBased,
				config: parentBasedConfig{
					Root:             traceIDRatioID,
					RemoteSampled:    alwaysOnID,
					RemoteNotSampled: alwaysOffID,
					LocalSampled:     alwaysOnID,
					LocalNotSampled:  alwaysOffID,
				},
			},
			expectedRootSamplerConfig: samplerConfig{
				samplerType: samplerTraceIDRatio,
				config: traceIDRatioConfig{
					samplingRateNumerator: samplingRateDenominator / 10,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			t.Setenv(tracesSamplerKey, c.sampler)

			if c.samplerArg != "" {
				t.Setenv(tracesSamplerArgKey, c.samplerArg)
			}

			sc, err := NewSamplingConfig(col)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, c.expectedSamplerID, sc.currentSamplerID)

			var activeSamplerIDInMap samplerID
			err = sc.probeActiveSampler.Lookup(uint32(0), &activeSamplerIDInMap)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, c.expectedSamplerID, activeSamplerIDInMap)

			var samplersConfigInMap samplerConfig
			err = sc.samplersConfig.Lookup(uint32(c.expectedSamplerID), &samplersConfigInMap)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, c.expectedSamplerConfig, samplersConfigInMap)

			parentBasedConfig, ok := samplersConfigInMap.config.(parentBasedConfig)
			if ok && c.samplerArg != "" {
				var baseConfig samplerConfig
				err = sc.samplersConfig.Lookup(uint32(parentBasedConfig.Root), &baseConfig)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, c.expectedRootSamplerConfig, baseConfig)
			}
		})
	}
}
