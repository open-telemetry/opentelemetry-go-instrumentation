// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"strconv"
	"strings"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
)

// Sampler decides whether a trace should be sampled and exported.
type Sampler interface {
	validate() error
	convert() (*sampling.Config, error)
}

// OpenTelemetry spec-defined sampler names and environment variables for configuration.
const (
	TracesSamplerKey    = "OTEL_TRACES_SAMPLER"
	TracesSamplerArgKey = "OTEL_TRACES_SAMPLER_ARG"

	SamplerNameAlwaysOn                = "always_on"
	SamplerNameAlwaysOff               = "always_off"
	SamplerNameTraceIDRatio            = "traceidratio"
	SamplerNameParentBasedAlwaysOn     = "parentbased_always_on"
	SamplerNameParsedBasedAlwaysOff    = "parentbased_always_off"
	SamplerNameParentBasedTraceIDRatio = "parentbased_traceidratio"
)

// AlwaysOn is a Sampler that samples every trace.
// Be careful about using this sampler in a production application with
// significant traffic: a new trace will be started and exported for every
// request.
type AlwaysOn struct{}

var _ Sampler = AlwaysOn{}

func (AlwaysOn) validate() error {
	return nil
}

func (AlwaysOn) convert() (*sampling.Config, error) {
	return &sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.AlwaysOnID: {
				SamplerType: sampling.SamplerAlwaysOn,
			},
		},
		ActiveSampler: sampling.AlwaysOnID,
	}, nil
}

// AlwaysOff returns a Sampler that samples no traces.
type AlwaysOff struct{}

var _ Sampler = AlwaysOff{}

func (AlwaysOff) validate() error {
	return nil
}

func (AlwaysOff) convert() (*sampling.Config, error) {
	return &sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.AlwaysOffID: {
				SamplerType: sampling.SamplerAlwaysOff,
			},
		},
		ActiveSampler: sampling.AlwaysOffID,
	}, nil
}

// TraceIDRatio samples a given fraction of traces. Fraction should be in the closed interval [0, 1].
// To respect the parent trace's SampledFlag, the TraceIDRatio sampler should be used
// as a delegate of a [ParentBased] sampler.
type TraceIDRatio struct {
	// Fraction is the fraction of traces to sample. This value needs to be in the interval [0, 1].
	Fraction float64
}

var _ Sampler = TraceIDRatio{}

func (t TraceIDRatio) validate() error {
	if t.Fraction < 0 || t.Fraction > 1 {
		return errors.New("fraction in TraceIDRatio must be in the range [0, 1]")
	}

	return nil
}

func (t TraceIDRatio) convert() (*sampling.Config, error) {
	tidConfig, err := sampling.NewTraceIDRatioConfig(t.Fraction)
	if err != nil {
		return nil, err
	}
	return &sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.TraceIDRatioID: {
				SamplerType: sampling.SamplerTraceIDRatio,
				Config:      tidConfig,
			},
		},
		ActiveSampler: sampling.TraceIDRatioID,
	}, nil
}

// ParentBased is a [Sampler] which behaves differently,
// based on the parent of the span. If the span has no parent,
// the Root sampler is used to make sampling decision. If the span has
// a parent, depending on whether the parent is remote and whether it
// is sampled, one of the following samplers will apply:
//   - RemoteSampled (default: [AlwaysOn])
//   - RemoteNotSampled (default: [AlwaysOff])
//   - LocalSampled (default: [AlwaysOn])
//   - LocalNotSampled (default: [AlwaysOff])
type ParentBased struct {
	// Root is the Sampler used when a span is created without a parent.
	Root Sampler
	// RemoteSampled is the Sampler used when the span parent is remote and sampled.
	RemoteSampled Sampler
	// RemoteNotSampled is the Sampler used when the span parent is remote and not sampled.
	RemoteNotSampled Sampler
	// LocalSampled is the Sampler used when the span parent is local and sampled.
	LocalSampled Sampler
	// LocalNotSampled is the Sampler used when the span parent is local and not sampled.
	LocalNotSampled Sampler
}

var _ Sampler = ParentBased{}

func validateParentBasedComponent(s Sampler) error {
	if s == nil {
		return nil
	}
	if _, ok := s.(ParentBased); ok {
		return errors.New("parent-based sampler cannot wrap parent-based sampler")
	}
	return s.validate()
}

func (p ParentBased) validate() error {
	var err error
	err = errors.Join(err, validateParentBasedComponent(p.Root))
	err = errors.Join(err, validateParentBasedComponent(p.RemoteSampled))
	err = errors.Join(err, validateParentBasedComponent(p.RemoteNotSampled))
	err = errors.Join(err, validateParentBasedComponent(p.LocalSampled))
	return errors.Join(err, validateParentBasedComponent(p.LocalNotSampled))
}

func (p ParentBased) convert() (*sampling.Config, error) {
	pbc := sampling.DefaultParentBasedSampler()
	samplers := make(map[sampling.SamplerID]sampling.SamplerConfig)
	rootSampler, err := ConvertSamplerToConfig(p.Root)
	if err != nil {
		return nil, err
	}
	if rootSampler != nil {
		pbc.Root = rootSampler.ActiveSampler
		for id, config := range rootSampler.Samplers {
			samplers[id] = config
		}
	}

	remoteSampledSampler, err := ConvertSamplerToConfig(p.RemoteSampled)
	if err != nil {
		return nil, err
	}
	if remoteSampledSampler != nil {
		pbc.RemoteSampled = remoteSampledSampler.ActiveSampler
		for id, config := range remoteSampledSampler.Samplers {
			samplers[id] = config
		}
	}

	remoteNotSampledSampler, err := ConvertSamplerToConfig(p.RemoteNotSampled)
	if err != nil {
		return nil, err
	}
	if remoteNotSampledSampler != nil {
		pbc.RemoteNotSampled = remoteNotSampledSampler.ActiveSampler
		for id, config := range remoteNotSampledSampler.Samplers {
			samplers[id] = config
		}
	}

	localSampledSamplers, err := ConvertSamplerToConfig(p.LocalSampled)
	if err != nil {
		return nil, err
	}
	if localSampledSamplers != nil {
		pbc.LocalSampled = localSampledSamplers.ActiveSampler
		for id, config := range localSampledSamplers.Samplers {
			samplers[id] = config
		}
	}

	localNotSampledSampler, err := ConvertSamplerToConfig(p.LocalNotSampled)
	if err != nil {
		return nil, err
	}
	if localNotSampledSampler != nil {
		pbc.LocalNotSampled = localNotSampledSampler.ActiveSampler
		for id, config := range localNotSampledSampler.Samplers {
			samplers[id] = config
		}
	}

	samplers[sampling.ParentBasedID] = sampling.SamplerConfig{
		SamplerType: sampling.SamplerParentBased,
		Config:      pbc,
	}

	return &sampling.Config{
		Samplers:      samplers,
		ActiveSampler: sampling.ParentBasedID,
	}, nil
}

// DefaultSampler returns a ParentBased sampler with the following defaults:
//   - Root: AlwaysOn
//   - RemoteSampled: AlwaysOn
//   - RemoteNotSampled: AlwaysOff
//   - LocalSampled: AlwaysOn
//   - LocalNotSampled: AlwaysOff
func DefaultSampler() Sampler {
	return ParentBased{
		Root:             AlwaysOn{},
		RemoteSampled:    AlwaysOn{},
		RemoteNotSampled: AlwaysOff{},
		LocalSampled:     AlwaysOn{},
		LocalNotSampled:  AlwaysOff{},
	}
}

// NewSamplerFromEnv creates a Sampler based on the environment variables.
// If the environment variables are not set, it returns a nil Sampler.
func NewSamplerFromEnv(lookupEnv func(string) (string, bool)) (Sampler, error) {
	samplerName, ok := lookupEnv(TracesSamplerKey)
	if !ok {
		return nil, nil
	}

	defaultSampler := DefaultSampler().(ParentBased)

	samplerName = strings.ToLower(strings.TrimSpace(samplerName))
	samplerArg, hasSamplerArg := lookupEnv(TracesSamplerArgKey)
	samplerArg = strings.TrimSpace(samplerArg)

	switch samplerName {
	case SamplerNameAlwaysOn:
		return AlwaysOn{}, nil
	case SamplerNameAlwaysOff:
		return AlwaysOff{}, nil
	case SamplerNameTraceIDRatio:
		if hasSamplerArg {
			ratio, err := strconv.ParseFloat(samplerArg, 64)
			if err != nil {
				return nil, err
			}
			return TraceIDRatio{Fraction: ratio}, nil
		}
		return TraceIDRatio{Fraction: 1}, nil
	case SamplerNameParentBasedAlwaysOn:
		defaultSampler.Root = AlwaysOn{}
		return defaultSampler, nil
	case SamplerNameParsedBasedAlwaysOff:
		defaultSampler.Root = AlwaysOff{}
		return defaultSampler, nil
	case SamplerNameParentBasedTraceIDRatio:
		if !hasSamplerArg {
			defaultSampler.Root = TraceIDRatio{Fraction: 1}
			return defaultSampler, nil
		}
		ratio, err := strconv.ParseFloat(samplerArg, 64)
		if err != nil {
			return nil, err
		}
		defaultSampler.Root = TraceIDRatio{Fraction: ratio}
		return defaultSampler, nil
	default:
		return nil, errors.New("unknown sampler name")
	}
}

// ConvertSamplerToConfig converts a Sampler its internal representation.
func ConvertSamplerToConfig(s Sampler) (*sampling.Config, error) {
	if s == nil {
		return nil, nil
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s.convert()
}
