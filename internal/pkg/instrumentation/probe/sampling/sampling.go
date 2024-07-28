// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
)

const (
	samplersConfigMapName     = "samplers_config_map"
	probeActiveSamplerMapName = "probe_active_sampler_map"
)

// SamplingConfig is used to configure the samplers used by eBPF.
type SamplingConfig struct {
	samplersConfig     *ebpf.Map
	probeActiveSampler *ebpf.Map

	currentSamplerID samplerID
}

type samplerType uint64

const (
	// OpenTelemetry spec-defined samplers.
	samplerAlwaysOn samplerType = iota
	samplerAlwaysOff
	samplerTraceIDRatio
	samplerParentBased
	// Custom samplers.
)

type traceIDRatioConfig struct {
	// samplingRateNumerator is the numerator of the sampling rate.
	// see samplingRateDenominator for more information.
	samplingRateNumerator uint64
}

// samplerID is a unique identifier for a sampler. It is used as a key in the samplers config map,
// and as a value in the active sampler map. In addition samplers can reference other samplers in their configuration by their ID.
type samplerID uint32

type parentBasedConfig struct {
	root             samplerID
	remoteSampled    samplerID
	remoteNotSampled samplerID
	localSampled     samplerID
	localNotSampled  samplerID
	_                [4]byte
}

// the following are constants which are used by the eBPF code.
// they should be kept in sync with the definitions there.
const (
	maxSampleConfigDataSize = 256
	sampleConfigSize        = maxSampleConfigDataSize + 8
	// since eBPF does not support floating point arithmetic, we use a rational number to represent the ratio.
	// the denominator is fixed and the numerator is used to represent the ratio.
	// This value can limit the precision of the sampling rate, hence setting it to a high value should be enough in terms of precision.
	samplingRateDenominator = 1e9
)

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

// config data for samplers is a union of all possible sampler configurations.
// the size of the data is fixed, and the actual configuration is stored in the first part of the data.
// the rest of the data is padding to make sure the size is fixed.

// The spec-defined samplers have a constant ID, and are always available.
const (
	alwaysOnID     samplerID = 0
	alwaysOffID    samplerID = 1
	traceIDRatioID samplerID = 2
	parentBasedID  samplerID = 3
)

type samplerConfig struct {
	samplerType samplerType
	config      any
}

func traceIDRationDefaultConfig() traceIDRatioConfig {
	return traceIDRatioConfig{samplingRateNumerator: samplingRateDenominator}
}

var _ encoding.BinaryMarshaler = &samplerConfig{}

func (sc *samplerConfig) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 0, sampleConfigSize)
	writingBuffer := bytes.NewBuffer(buf)

	err := binary.Write(writingBuffer, binary.NativeEndian, sc.samplerType)
	if err != nil {
		return nil, err
	}

	err = binary.Write(writingBuffer, binary.NativeEndian, sc.config)
	if err != nil {
		return nil, err
	}

	if available := writingBuffer.Available(); available > 0 {
		_, _ = writingBuffer.Write(make([]byte, available))
	}

	return writingBuffer.Bytes(), nil
}

// NewSamplingConfig creates a new SamplingConfig from the given eBPF collection.
// It applies the sampler configuration from the environment variables.
func NewSamplingConfig(c *ebpf.Collection) (*SamplingConfig, error) {
	samplersConfig, ok := c.Maps[samplersConfigMapName]
	if !ok {
		return nil, fmt.Errorf("map %s not found", samplersConfigMapName)
	}

	probeActiveSampler, ok := c.Maps[probeActiveSamplerMapName]
	if !ok {
		return nil, fmt.Errorf("map %s not found", probeActiveSamplerMapName)
	}
	
	sc := &SamplingConfig{
		samplersConfig:     samplersConfig,
		probeActiveSampler: probeActiveSampler,
	}

	err := sc.applySamplerFromEnv()
	if err != nil {
		return nil, err
	}

	return sc, nil
}

func defaultParentBasedSampler() parentBasedConfig {
	return parentBasedConfig{
		root:             alwaysOnID,
		remoteSampled:    alwaysOnID,
		remoteNotSampled: alwaysOffID,
		localSampled:     alwaysOnID,
		localNotSampled:  alwaysOffID,
	}
}


// taken from go.opentelemetry.io/otel/sdk/trace and adapted to work with eBPF
func (sc *SamplingConfig) applySamplerFromEnv() error {
	defaultParentBased := defaultParentBasedSampler()
	defaultTraceIDRatio := samplerConfig{
		samplerType: samplerTraceIDRatio,
		config: traceIDRationDefaultConfig(),
	}

	sampler, ok := os.LookupEnv(tracesSamplerKey)
	if !ok {
		// default to parent-based sampler with always_on root
		err := sc.setSamplerConfig(&samplerConfig{samplerType: samplerParentBased, config: defaultParentBased}, parentBasedID)
		if err != nil {
			return err
		}
		return sc.setActiveSampler(parentBasedID)
	}

	sampler = strings.ToLower(strings.TrimSpace(sampler))
	samplerArg, hasSamplerArg := os.LookupEnv(tracesSamplerArgKey)
	samplerArg = strings.TrimSpace(samplerArg)

	var samplerID samplerID
	var err error

	switch sampler {
	case samplerNameAlwaysOn:
		samplerID = alwaysOnID
	case samplerNameAlwaysOff:
		samplerID = alwaysOffID
	case samplerNameTraceIDRatio:
		if hasSamplerArg {
			if defaultTraceIDRatio.config, err = parseTraceIDRatio(samplerArg); err != nil {
				break
			}
		}
		err = sc.setSamplerConfig(&defaultTraceIDRatio, traceIDRatioID)
		samplerID = traceIDRatioID
	case samplerNameParentBasedAlwaysOn:
		defaultParentBased.root = alwaysOnID
		err = sc.setSamplerConfig(&samplerConfig{samplerType: samplerParentBased, config: defaultParentBased}, parentBasedID)
		samplerID = parentBasedID
	case samplerNameParsedBasedAlwaysOff:
		defaultParentBased.root = alwaysOffID
		err = sc.setSamplerConfig(&samplerConfig{samplerType: samplerParentBased, config: defaultParentBased}, parentBasedID)
		samplerID = parentBasedID
	case samplerNameParentBasedTraceIDRatio:
		defaultParentBased.root = traceIDRatioID
		if hasSamplerArg {
			if defaultTraceIDRatio.config, err = parseTraceIDRatio(samplerArg); err != nil {
				break
			}
		}
		err = sc.setSamplerConfig(&defaultTraceIDRatio, traceIDRatioID)
		if err == nil {
			err = sc.setSamplerConfig(&samplerConfig{samplerType: samplerParentBased, config: defaultParentBased}, parentBasedID)
		}
		samplerID = parentBasedID
	default:
		samplerID = parentBasedID
	}

	if err != nil {
		return err
	}

	err = sc.setActiveSampler(samplerID)
	return err
}

type samplerArgParseError struct {
	parseErr error
}

func (e samplerArgParseError) Error() string {
	return fmt.Sprintf("parsing sampler argument: %s", e.parseErr.Error())
}

func parseTraceIDRatio(ratio string) (traceIDRatioConfig, error) {
	v, err := strconv.ParseFloat(ratio, 64)
	if err != nil {
		return traceIDRationDefaultConfig(), samplerArgParseError{err}
	}

	numerator, err := floatToNumerator(v, samplingRateDenominator)
	if err != nil {
		return traceIDRationDefaultConfig(), samplerArgParseError{err}
	}

	return traceIDRatioConfig{samplingRateNumerator: numerator}, nil
}

func (sc *SamplingConfig) setSamplerConfig(samplerConfig *samplerConfig, id samplerID) error {
	return sc.samplersConfig.Put(id, samplerConfig)
}

func (sc *SamplingConfig) setActiveSampler(id samplerID) error {
	err := sc.probeActiveSampler.Put(uint32(0), id)
	if err != nil {
		return err
	}

	sc.currentSamplerID = id
	return nil
}
