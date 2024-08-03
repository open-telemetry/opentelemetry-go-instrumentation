// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"errors"
	"strconv"
	"strings"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
)

// Sampler decides whether a trace should be sampled and exported.
type Sampler interface {
	validate() error
	convert() (sampling.Config, error)
}

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

// AlwaysOn is a Sampler that samples every trace.
// Be careful about using this sampler in a production application with
// significant traffic: a new trace will be started and exported for every
// request.
type AlwaysOn struct{}

var _ Sampler = AlwaysOn{}

func (AlwaysOn) validate() error {
	return nil
}

func (AlwaysOn) convert() (sampling.Config, error) {
	return sampling.Config{
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

func (AlwaysOff) convert() (sampling.Config, error) {
	return sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.AlwaysOffID: {
				SamplerType: sampling.SamplerAlwaysOff,
			},
		},
		ActiveSampler: sampling.AlwaysOffID,
	}, nil
}

// TraceIDRatio samples a given fraction of traces. Fractions should be in the close interval [0, 1].
// To respect the parent trace's `SampledFlag`, the `TraceIDRatio` sampler should be used
// as a delegate of a `Parent` sampler.
type TraceIDRatio struct {
	Fraction float64
}

var _ Sampler = TraceIDRatio{}

func (t TraceIDRatio) validate() error {
	if t.Fraction < 0 || t.Fraction > 1 {
		return errors.New("fraction in TraceIDRatio must be in the range [0, 1]")
	}

	return nil
}

func (t TraceIDRatio) convert() (sampling.Config, error) {
	tidConfig, err := sampling.NewTraceIDRationConfig(t.Fraction)
	if err != nil {
		return sampling.Config{}, err
	}
	return sampling.Config{
		Samplers: map[sampling.SamplerID]sampling.SamplerConfig{
			sampling.TraceIDRatioID: {
				SamplerType: sampling.SamplerTraceIDRatio,
				Config:      tidConfig,
			},
		},
		ActiveSampler: sampling.TraceIDRatioID,
	}, nil
}

// ParentBased returns a sampler decorator which behaves differently,
// based on the parent of the span. If the span has no parent,
// the `Root` sampler is used to make sampling decision. If the span has
// a parent, depending on whether the parent is remote and whether it
// is sampled, one of the following samplers will apply:
//   - RemoteSampled (default: AlwaysOn)
//   - RemoteNotSampled (default: AlwaysOff)
//   - LocalSampled (default: AlwaysOn)
//   - LocalNotSampled (default: AlwaysOff)
type ParentBased struct {
	Root             Sampler
	RemoteSampled    Sampler
	RemoteNotSampled Sampler
	LocalSampled     Sampler
	LocalNotSampled  Sampler
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
	if err := validateParentBasedComponent(p.Root); err != nil {
		return err
	}
	if err := validateParentBasedComponent(p.RemoteSampled); err != nil {
		return err
	}
	if err := validateParentBasedComponent(p.RemoteNotSampled); err != nil {
		return err
	}
	if err := validateParentBasedComponent(p.LocalSampled); err != nil {
		return err
	}
	return validateParentBasedComponent(p.LocalNotSampled)
}

func getSamplerConfig(s Sampler) (sampling.Config, error) {
	if s == nil {
		return sampling.Config{}, nil
	}
	config, err := s.convert()
	if err != nil {
		return sampling.Config{}, err
	}
	return config, nil
}

func (p ParentBased) convert() (sampling.Config, error) {
	pbc := sampling.DefaultParentBasedSampler()
	samplers := make(map[sampling.SamplerID]sampling.SamplerConfig)
	rootSampler, err := getSamplerConfig(p.Root)
	if err != nil {
		return sampling.Config{}, err
	}
	if !rootSampler.IsZero() {
		pbc.Root = rootSampler.ActiveSampler
		for id, config := range rootSampler.Samplers {
			if config.Config != nil {
				samplers[id] = config
			}
		}
	}

	remoteSampledSampler, err := getSamplerConfig(p.RemoteSampled)
	if err != nil {
		return sampling.Config{}, err
	}
	if !remoteSampledSampler.IsZero() {
		pbc.RemoteSampled = remoteSampledSampler.ActiveSampler
		for id, config := range remoteSampledSampler.Samplers {
			if config.Config != nil {
				samplers[id] = config
			}
		}
	}

	remoteNotSampledSampler, err := getSamplerConfig(p.RemoteNotSampled)
	if err != nil {
		return sampling.Config{}, err
	}
	if !remoteNotSampledSampler.IsZero() {
		pbc.RemoteNotSampled = remoteNotSampledSampler.ActiveSampler
		for id, config := range remoteNotSampledSampler.Samplers {
			if config.Config != nil {
				samplers[id] = config
			}
		}
	}

	localSampledSamplers, err := getSamplerConfig(p.LocalSampled)
	if err != nil {
		return sampling.Config{}, err
	}
	if !localSampledSamplers.IsZero() {
		pbc.LocalSampled = localSampledSamplers.ActiveSampler
		for id, config := range localSampledSamplers.Samplers {
			if config.Config != nil {
				samplers[id] = config
			}
		}
	}

	localNotSampledSampler, err := getSamplerConfig(p.LocalNotSampled)
	if err != nil {
		return sampling.Config{}, err
	}
	if !localNotSampledSampler.IsZero() {
		pbc.LocalNotSampled = localNotSampledSampler.ActiveSampler
		for id, config := range localNotSampledSampler.Samplers {
			if config.Config != nil {
				samplers[id] = config
			}
		}
	}

	samplers[sampling.ParentBasedID] = sampling.SamplerConfig{
		SamplerType: sampling.SamplerParentBased,
		Config:      pbc,
	}

	return sampling.Config{
		Samplers:      samplers,
		ActiveSampler: sampling.ParentBasedID,
	}, nil
}

func newSamplerFromEnv() (Sampler, error) {
	defaultSampler := ParentBased{
		Root:             AlwaysOn{},
		RemoteSampled:    AlwaysOn{},
		RemoteNotSampled: AlwaysOff{},
		LocalSampled:     AlwaysOn{},
		LocalNotSampled:  AlwaysOff{},
	}

	samplerName, ok := lookupEnv(tracesSamplerKey)
	if !ok {
		return defaultSampler, nil
	}

	samplerName = strings.ToLower(strings.TrimSpace(samplerName))
	samplerArg, hasSamplerArg := lookupEnv(tracesSamplerArgKey)
	samplerArg = strings.TrimSpace(samplerArg)

	switch samplerName {
	case samplerNameAlwaysOn:
		return AlwaysOn{}, nil
	case samplerNameAlwaysOff:
		return AlwaysOff{}, nil
	case samplerNameTraceIDRatio:
		if hasSamplerArg {
			ratio, err := parseTraceIDRatio(samplerArg)
			if err != nil {
				return nil, err
			}
			return TraceIDRatio{Fraction: ratio}, nil
		}
		return TraceIDRatio{Fraction: 1}, nil
	case samplerNameParentBasedAlwaysOn:
		defaultSampler.Root = AlwaysOn{}
		return defaultSampler, nil
	case samplerNameParsedBasedAlwaysOff:
		defaultSampler.Root = AlwaysOff{}
		return defaultSampler, nil
	case samplerNameParentBasedTraceIDRatio:
		if !hasSamplerArg {
			defaultSampler.Root = TraceIDRatio{Fraction: 1}
			return defaultSampler, nil
		}
		ratio, err := parseTraceIDRatio(samplerArg)
		if err != nil {
			return nil, err
		}
		defaultSampler.Root = TraceIDRatio{Fraction: ratio}
		return defaultSampler, nil
	default:
		return nil, errors.New("unknown sampler name")
	}
}

func parseTraceIDRatio(s string) (float64, error) {
	ratio, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return ratio, nil
}
