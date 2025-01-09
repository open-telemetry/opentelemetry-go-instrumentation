// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ebpf_test
//go:build multi_kernel_test
package sampling

import (
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/stretchr/testify/assert"
)

func mockEBPFCollectionForSampling() (*ebpf.Collection, error) {
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

func TestEBPFNewSamplingConfigDefault(t *testing.T) {
	c, err := mockEBPFCollectionForSampling()
	if !assert.NoError(t, err) {
		return
	}

	defer c.Close()
	t.Run("default", func(t *testing.T) {
		m, err := NewSamplingManager(c, DefaultConfig())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, m.currentSamplerID, ParentBasedID)

		var activeSamplerIDInMap SamplerID
		err = m.ActiveSamplerMap.Lookup(uint32(0), &activeSamplerIDInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, activeSamplerIDInMap, ParentBasedID)

		var samplersConfigInMap SamplerConfig
		err = m.samplersConfigMap.Lookup(uint32(ParentBasedID), &samplersConfigInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, samplersConfigInMap.SamplerType, SamplerParentBased)
		assert.Equal(t, samplersConfigInMap.Config, DefaultParentBasedSampler())
	})

	t.Run("parent based with trace id ratio", func(t *testing.T) {
		pb := DefaultParentBasedSampler()
		pb.Root = TraceIDRatioID
		m, err := NewSamplingManager(c, &Config{
			Samplers: map[SamplerID]SamplerConfig{
				ParentBasedID: {
					SamplerType: SamplerParentBased,
					Config:      pb,
				},
				TraceIDRatioID: {
					SamplerType: SamplerTraceIDRatio,
					Config:      TraceIDRatioConfig{42},
				},
			},
			ActiveSampler: ParentBasedID,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, ParentBasedID, m.currentSamplerID)

		var activeSamplerIDInMap SamplerID
		err = m.ActiveSamplerMap.Lookup(uint32(0), &activeSamplerIDInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, ParentBasedID, activeSamplerIDInMap)

		var samplersConfigInMap SamplerConfig
		err = m.samplersConfigMap.Lookup(uint32(ParentBasedID), &samplersConfigInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, SamplerParentBased, samplersConfigInMap.SamplerType)
		assert.Equal(t, pb, samplersConfigInMap.Config)

		err = m.samplersConfigMap.Lookup(uint32(TraceIDRatioID), &samplersConfigInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, SamplerTraceIDRatio, samplersConfigInMap.SamplerType)
		assert.Equal(t, TraceIDRatioConfig{42}, samplersConfigInMap.Config)
	})

	t.Run("nil config", func(t *testing.T) {
		m, err := NewSamplingManager(c, nil)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, m.currentSamplerID, ParentBasedID)

		var activeSamplerIDInMap SamplerID
		err = m.ActiveSamplerMap.Lookup(uint32(0), &activeSamplerIDInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, activeSamplerIDInMap, ParentBasedID)

		var samplersConfigInMap SamplerConfig
		err = m.samplersConfigMap.Lookup(uint32(ParentBasedID), &samplersConfigInMap)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, samplersConfigInMap.SamplerType, SamplerParentBased)
		assert.Equal(t, samplersConfigInMap.Config, DefaultParentBasedSampler())
	})
}
