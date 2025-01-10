// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/cilium/ebpf"
)

// SamplerType defines the type of a sampler.
type SamplerType uint64

// OpenTelemetry spec-defined samplers.
const (
	SamplerAlwaysOn SamplerType = iota
	SamplerAlwaysOff
	SamplerTraceIDRatio
	SamplerParentBased

	// Custom samplers TODO.
)

type TraceIDRatioConfig struct {
	// samplingRateNumerator is the numerator of the sampling rate.
	// see samplingRateDenominator for more information.
	samplingRateNumerator uint64
}

func NewTraceIDRatioConfig(ratio float64) (TraceIDRatioConfig, error) {
	numerator, err := floatToNumerator(ratio, samplingRateDenominator)
	if err != nil {
		return TraceIDRatioConfig{}, err
	}
	return TraceIDRatioConfig{numerator}, nil
}

// SamplerID is a unique identifier for a sampler. It is used as a key in the samplers config map,
// and as a value in the active sampler map. In addition samplers can reference other samplers in their configuration by their ID.
type SamplerID uint32

// Config holds the configuration for the eBPF samplers.
type Config struct {
	// Samplers is a map of sampler IDs to their configuration.
	Samplers map[SamplerID]SamplerConfig
	// ActiveSampler is the ID of the currently active sampler.
	// The active sampler id must be one of the keys in the samplers map.
	// Each sampler can reference other samplers in their configuration by their ID.
	// When referencing another sampler, the ID must be one of the keys in the samplers map.
	ActiveSampler SamplerID
}

func DefaultConfig() *Config {
	return &Config{
		Samplers: map[SamplerID]SamplerConfig{
			AlwaysOnID: {
				SamplerType: SamplerAlwaysOn,
			},
			AlwaysOffID: {
				SamplerType: SamplerAlwaysOff,
			},
			ParentBasedID: {
				SamplerType: SamplerParentBased,
				Config:      DefaultParentBasedSampler(),
			},
		},
		ActiveSampler: ParentBasedID,
	}
}

// ParentBasedConfig holds the configuration for the ParentBased sampler.
type ParentBasedConfig struct {
	Root             SamplerID
	RemoteSampled    SamplerID
	RemoteNotSampled SamplerID
	LocalSampled     SamplerID
	LocalNotSampled  SamplerID
}

// the following are constants which are used by the eBPF code.
// they should be kept in sync with the definitions there.
const (
	maxSampleConfigDataSize = 256
	sampleConfigSize        = maxSampleConfigDataSize + 8
	// since eBPF does not support floating point arithmetic, we use a rational number to represent the ratio.
	// the denominator is fixed and the numerator is used to represent the ratio.
	// This value can limit the precision of the sampling rate, hence setting it to a high value should be enough in terms of precision.
	samplingRateDenominator = math.MaxUint32
	maxSamplers             = 32
)

// The spec-defined samplers have a constant ID, and are always available.
const (
	AlwaysOnID     SamplerID = 0
	AlwaysOffID    SamplerID = 1
	TraceIDRatioID SamplerID = 2
	ParentBasedID  SamplerID = 3
)

// SamplerConfig holds the configuration for a specific sampler. data for samplers is a union of all possible sampler configurations.
// the size of the data is fixed, and the actual configuration is stored in the first part of the data.
// the rest of the data is padding to make sure the size is fixed.
type SamplerConfig struct {
	SamplerType SamplerType
	Config      any
}

func (sc *SamplerConfig) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 0, sampleConfigSize)
	writingBuffer := bytes.NewBuffer(buf)

	err := binary.Write(writingBuffer, binary.NativeEndian, sc.SamplerType)
	if err != nil {
		return nil, err
	}

	if sc.Config != nil {
		// sampler config may be empty. In that case, we don't write anything.
		err = binary.Write(writingBuffer, binary.NativeEndian, sc.Config)
		if err != nil {
			return nil, err
		}
	}

	if available := writingBuffer.Available(); available > 0 {
		_, _ = writingBuffer.Write(make([]byte, available))
	}

	return writingBuffer.Bytes(), nil
}

func (sc *SamplerConfig) UnmarshalBinary(data []byte) error {
	if len(data) != sampleConfigSize {
		return fmt.Errorf("invalid data size for sampler config: %d", len(data))
	}
	readingBuffer := bytes.NewReader(data)

	err := binary.Read(readingBuffer, binary.NativeEndian, &sc.SamplerType)
	if err != nil {
		return err
	}

	switch sc.SamplerType {
	case SamplerAlwaysOn, SamplerAlwaysOff:
		return nil
	case SamplerTraceIDRatio:
		var numerator uint64
		err := binary.Read(readingBuffer, binary.NativeEndian, &numerator)
		if err != nil {
			return err
		}
		sc.Config = TraceIDRatioConfig{numerator}
	case SamplerParentBased:
		var parentBased ParentBasedConfig
		err := binary.Read(readingBuffer, binary.NativeEndian, &parentBased)
		if err != nil {
			return err
		}
		sc.Config = parentBased
	}

	return nil
}

// Manager is used to configure the samplers used by eBPF.
type Manager struct {
	samplersConfigMap *ebpf.Map
	ActiveSamplerMap  *ebpf.Map

	currentSamplerID SamplerID
}

const (
	samplersConfigMapName     = "samplers_config_map"
	probeActiveSamplerMapName = "probe_active_sampler_map"
)

func DefaultParentBasedSampler() ParentBasedConfig {
	return ParentBasedConfig{
		Root:             AlwaysOnID,
		RemoteSampled:    AlwaysOnID,
		RemoteNotSampled: AlwaysOffID,
		LocalSampled:     AlwaysOnID,
		LocalNotSampled:  AlwaysOffID,
	}
}

// NewSamplingManager creates a new Manager from the given eBPF collection with the given configuration.
func NewSamplingManager(c *ebpf.Collection, conf *Config) (*Manager, error) {
	samplersConfig, ok := c.Maps[samplersConfigMapName]
	if !ok {
		return nil, fmt.Errorf("map %s not found", samplersConfigMapName)
	}

	probeActiveSampler, ok := c.Maps[probeActiveSamplerMapName]
	if !ok {
		return nil, fmt.Errorf("map %s not found", probeActiveSamplerMapName)
	}

	m := &Manager{
		samplersConfigMap: samplersConfig,
		ActiveSamplerMap:  probeActiveSampler,
	}

	if conf == nil {
		conf = DefaultConfig()
	}

	err := m.applyConfig(conf)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) applyConfig(conf *Config) error {
	if conf == nil {
		return fmt.Errorf("cannot apply nil config")
	}

	samplerIDs := make([]SamplerID, 0, len(conf.Samplers))
	configs := make([]SamplerConfig, 0, len(conf.Samplers))
	for id, samplerConfig := range conf.Samplers {
		samplerIDs = append(samplerIDs, id)
		configs = append(configs, samplerConfig)
	}

	configsBytes := make([][sampleConfigSize]byte, len(configs))
	for i, config := range configs {
		b, err := config.MarshalBinary()
		if err != nil {
			return err
		}
		if len(b) != sampleConfigSize {
			return fmt.Errorf("unexpected sampler config size, expected %d, got %d", sampleConfigSize, len(b))
		}
		copy(configsBytes[i][:], b)
	}

	_, err := m.samplersConfigMap.BatchUpdate(samplerIDs, configsBytes, &ebpf.BatchOptions{})
	if err != nil {
		if !errors.Is(err, ebpf.ErrNotSupported) {
			return fmt.Errorf("failed to update samplers config map: %w", err)
		}
		// batch update is supported for kernels >= 5.6
		// fallback to single updates
		for i := range samplerIDs {
			err = m.samplersConfigMap.Put(uint32(samplerIDs[i]), configsBytes[i])
			if err != nil {
				return err
			}
		}
	}

	err = m.setActiveSampler(conf.ActiveSampler)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) setActiveSampler(id SamplerID) error {
	err := m.ActiveSamplerMap.Put(uint32(0), id)
	if err != nil {
		return err
	}

	m.currentSamplerID = id
	return nil
}
