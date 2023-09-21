// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auto

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// envTargetExeKey is the key for the environment variable value pointing to the
// target binary to instrument.
const envTargetExeKey = "OTEL_GO_AUTO_TARGET_EXE"

// Instrumentation manages and controls all OpenTelemetry Go
// auto-instrumentation.
type Instrumentation struct {
	target   *process.TargetDetails
	analyzer *process.Analyzer
	manager  *instrumentors.Manager
}

var (
	// Error message returned when instrumentation is launched without a target
	// binary.
	errUndefinedTarget = fmt.Errorf("undefined target Go binary, consider setting the %s environment variable pointing to the target binary to instrument", envTargetExeKey)
)

// NewInstrumentation returns a new [Instrumentation] configured with the
// provided opts.
func NewInstrumentation(opts ...InstrumentationOption) (*Instrumentation, error) {
	c := newInstConfig(opts)
	if err := c.validate(); err != nil {
		return nil, err
	}

	pa := process.NewAnalyzer()
	pid, err := pa.DiscoverProcessID(c.target)
	if err != nil {
		return nil, err
	}

	ctrl, err := opentelemetry.NewController(Version())
	if err != nil {
		return nil, err
	}

	mngr, err := instrumentors.NewManager(ctrl)
	if err != nil {
		return nil, err
	}

	td, err := pa.Analyze(pid, mngr.GetRelevantFuncs())
	if err != nil {
		mngr.Close()
		return nil, err
	}
	log.Logger.V(0).Info(
		"target process analysis completed",
		"pid", td.PID,
		"go_version", td.GoVersion,
		"dependencies", td.Libraries,
		"total_functions_found", len(td.Functions),
	)
	mngr.FilterUnusedInstrumentors(td)

	return &Instrumentation{
		target:   td,
		analyzer: pa,
		manager:  mngr,
	}, nil
}

// Run starts the instrumentation.
func (i *Instrumentation) Run(ctx context.Context) error {
	return i.manager.Run(ctx, i.target)
}

// Close closes the Instrumentation, cleaning up all used resources.
func (i *Instrumentation) Close() error {
	i.analyzer.Close()
	i.manager.Close()
	return nil
}

// InstrumentationOption applies a configuration option to [Instrumentation].
type InstrumentationOption interface {
	apply(instConfig) instConfig
}

type instConfig struct {
	target *process.TargetArgs
}

func newInstConfig(opts []InstrumentationOption) instConfig {
	var c instConfig
	for _, opt := range opts {
		c = opt.apply(c)
	}
	c = c.applyEnv()
	return c
}

func (c instConfig) applyEnv() instConfig {
	if v, ok := os.LookupEnv(envTargetExeKey); ok {
		c.target = &process.TargetArgs{ExePath: v}
	}
	return c
}

func (c instConfig) validate() error {
	if c.target == nil {
		return errUndefinedTarget
	}
	return c.target.Validate()
}

type fnOpt func(instConfig) instConfig

func (o fnOpt) apply(c instConfig) instConfig { return o(c) }

// WithTarget returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being executed at the provided path.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined it will take precedence over any value
// passed here.
func WithTarget(path string) InstrumentationOption {
	return fnOpt(func(c instConfig) instConfig {
		c.target = &process.TargetArgs{ExePath: path}
		return c
	})
}
