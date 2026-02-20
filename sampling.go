// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"errors"
	"maps"
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
	tracesSamplerKey    = "OTEL_TRACES_SAMPLER"
	tracesSamplerArgKey = "OTEL_TRACES_SAMPLER_ARG"

	samplerNameAlwaysOn                = "always_on"
	samplerNameAlwaysOff               = "always_off"
	samplerNameTraceIDRatio            = "traceidratio"
	samplerNameParentBasedAlwaysOn     = "parentbased_always_on"
	samplerNameParsedBasedAlwaysOff    = "parentbased_always_off"
	samplerNameParentBasedTraceIDRatio = "parentbased_traceidratio"
)

// AlwaysOnSampler is a Sampler that samples every trace.
// Be careful about using this sampler in a production application with
// significant traffic: a new trace will be started and exported for every
// request.
type AlwaysOnSampler struct{}

var _ Sampler = AlwaysOnSampler{}

func (AlwaysOnSampler) validate() error {
	return nil
}

func (AlwaysOnSampler) convert() (*sampling.Config, error) {
	return &sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.AlwaysOnID: {
				SamplerType: sampling.SamplerAlwaysOn,
			},
		},
		ActiveSampler: sampling.AlwaysOnID,
	}, nil
}

// AlwaysOffSampler returns a Sampler that samples no traces.
type AlwaysOffSampler struct{}

var _ Sampler = AlwaysOffSampler{}

func (AlwaysOffSampler) validate() error {
	return nil
}

func (AlwaysOffSampler) convert() (*sampling.Config, error) {
	return &sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.AlwaysOffID: {
				SamplerType: sampling.SamplerAlwaysOff,
			},
		},
		ActiveSampler: sampling.AlwaysOffID,
	}, nil
}

// TraceIDRatioSampler samples a given fraction of traces. Fraction should be in the closed interval [0, 1].
// To respect the parent trace's SampledFlag, the TraceIDRatioSampler sampler should be used
// as a delegate of a [ParentBased] sampler.
type TraceIDRatioSampler struct {
	// Fraction is the fraction of traces to sample. This value needs to be in the interval [0, 1].
	Fraction float64
}

var _ Sampler = TraceIDRatioSampler{}

func (t TraceIDRatioSampler) validate() error {
	if t.Fraction < 0 || t.Fraction > 1 {
		return errors.New("fraction in TraceIDRatio must be in the range [0, 1]")
	}

	return nil
}

func (t TraceIDRatioSampler) convert() (*sampling.Config, error) {
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

// ParentBasedSampler is a [Sampler] which behaves differently,
// based on the parent of the span. If the span has no parent,
// the Root sampler is used to make sampling decision. If the span has
// a parent, depending on whether the parent is remote and whether it
// is sampled, one of the following samplers will apply:
//   - RemoteSampled (default: [AlwaysOn])
//   - RemoteNotSampled (default: [AlwaysOff])
//   - LocalSampled (default: [AlwaysOn])
//   - LocalNotSampled (default: [AlwaysOff])
type ParentBasedSampler struct {
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

var _ Sampler = ParentBasedSampler{}

func validateParentBasedComponent(s Sampler) error {
	if s == nil {
		return nil
	}
	if _, ok := s.(ParentBasedSampler); ok {
		return errors.New("parent-based sampler cannot wrap parent-based sampler")
	}
	return s.validate()
}

func (p ParentBasedSampler) validate() error {
	var err error
	return errors.Join(err,
		validateParentBasedComponent(p.LocalNotSampled),
		validateParentBasedComponent(p.LocalSampled),
		validateParentBasedComponent(p.RemoteNotSampled),
		validateParentBasedComponent(p.RemoteSampled),
		validateParentBasedComponent(p.Root))
}

func (p ParentBasedSampler) convert() (*sampling.Config, error) {
	pbc := sampling.DefaultParentBasedSampler()
	samplers := make(map[sampling.SamplerID]sampling.SamplerConfig)
	rootSampler, err := convertSamplerToConfig(p.Root)
	if err != nil {
		return nil, err
	}
	if rootSampler != nil {
		pbc.Root = rootSampler.ActiveSampler
		maps.Copy(samplers, rootSampler.Samplers)
	}

	remoteSampledSampler, err := convertSamplerToConfig(p.RemoteSampled)
	if err != nil {
		return nil, err
	}
	if remoteSampledSampler != nil {
		pbc.RemoteSampled = remoteSampledSampler.ActiveSampler
		maps.Copy(samplers, remoteSampledSampler.Samplers)
	}

	remoteNotSampledSampler, err := convertSamplerToConfig(p.RemoteNotSampled)
	if err != nil {
		return nil, err
	}
	if remoteNotSampledSampler != nil {
		pbc.RemoteNotSampled = remoteNotSampledSampler.ActiveSampler
		maps.Copy(samplers, remoteNotSampledSampler.Samplers)
	}

	localSampledSamplers, err := convertSamplerToConfig(p.LocalSampled)
	if err != nil {
		return nil, err
	}
	if localSampledSamplers != nil {
		pbc.LocalSampled = localSampledSamplers.ActiveSampler
		maps.Copy(samplers, localSampledSamplers.Samplers)
	}

	localNotSampledSampler, err := convertSamplerToConfig(p.LocalNotSampled)
	if err != nil {
		return nil, err
	}
	if localNotSampledSampler != nil {
		pbc.LocalNotSampled = localNotSampledSampler.ActiveSampler
		maps.Copy(samplers, localNotSampledSampler.Samplers)
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
	return ParentBasedSampler{
		Root:             AlwaysOnSampler{},
		RemoteSampled:    AlwaysOnSampler{},
		RemoteNotSampled: AlwaysOffSampler{},
		LocalSampled:     AlwaysOnSampler{},
		LocalNotSampled:  AlwaysOffSampler{},
	}
}

// newSamplerFromEnv creates a Sampler based on the environment variables.
// If the environment variables are not set, it returns a nil Sampler.
func newSamplerFromEnv(lookupEnv func(string) (string, bool)) (Sampler, error) {
	samplerName, ok := lookupEnv(tracesSamplerKey)
	if !ok {
		return nil, nil
	}

	defaultSampler := DefaultSampler().(ParentBasedSampler)

	samplerName = strings.ToLower(strings.TrimSpace(samplerName))
	samplerArg, hasSamplerArg := lookupEnv(tracesSamplerArgKey)
	samplerArg = strings.TrimSpace(samplerArg)

	switch samplerName {
	case samplerNameAlwaysOn:
		return AlwaysOnSampler{}, nil
	case samplerNameAlwaysOff:
		return AlwaysOffSampler{}, nil
	case samplerNameTraceIDRatio:
		if hasSamplerArg {
			ratio, err := strconv.ParseFloat(samplerArg, 64)
			if err != nil {
				return nil, err
			}
			return TraceIDRatioSampler{Fraction: ratio}, nil
		}
		return TraceIDRatioSampler{Fraction: 1}, nil
	case samplerNameParentBasedAlwaysOn:
		defaultSampler.Root = AlwaysOnSampler{}
		return defaultSampler, nil
	case samplerNameParsedBasedAlwaysOff:
		defaultSampler.Root = AlwaysOffSampler{}
		return defaultSampler, nil
	case samplerNameParentBasedTraceIDRatio:
		if !hasSamplerArg {
			defaultSampler.Root = TraceIDRatioSampler{Fraction: 1}
			return defaultSampler, nil
		}
		ratio, err := strconv.ParseFloat(samplerArg, 64)
		if err != nil {
			return nil, err
		}
		defaultSampler.Root = TraceIDRatioSampler{Fraction: ratio}
		return defaultSampler, nil
	default:
		return nil, errors.New("unknown sampler name")
	}
}

// convertSamplerToConfig converts a Sampler its internal representation.
func convertSamplerToConfig(s Sampler) (*sampling.Config, error) {
	if s == nil {
		return nil, nil
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s.convert()
}
