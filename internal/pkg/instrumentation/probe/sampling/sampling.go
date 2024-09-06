// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import "math"

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
	// since eBPF does not support floating point arithmetic, we use a rational number to represent the ratio.
	// the denominator is fixed and the numerator is used to represent the ratio.
	// This value can limit the precision of the sampling rate, hence setting it to a high value should be enough in terms of precision.
	samplingRateDenominator = math.MaxUint32
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

func DefaultParentBasedSampler() ParentBasedConfig {
	return ParentBasedConfig{
		Root:             AlwaysOnID,
		RemoteSampled:    AlwaysOnID,
		RemoteNotSampled: AlwaysOffID,
		LocalSampled:     AlwaysOnID,
		LocalNotSampled:  AlwaysOffID,
	}
}
