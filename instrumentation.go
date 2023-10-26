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
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/go-logr/zapr"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/zap"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

const (
	// envTargetExeKey is the key for the environment variable value pointing to the
	// target binary to instrument.
	envTargetExeKey = "OTEL_GO_AUTO_TARGET_EXE"
	// envServiceName is the key for the envoriment variable value containing the service name.
	envServiceNameKey = "OTEL_SERVICE_NAME"
	// envResourceAttrKey is the key for the environment variable value containing
	// OpenTelemetry Resource attributes.
	envResourceAttrKey = "OTEL_RESOURCE_ATTRIBUTES"
	// envTracesExportersKey is the key for the environment variable value
	// containing what OpenTelemetry trace exporter to use.
	envTracesExportersKey = "OTEL_TRACES_EXPORTER"
)

// Instrumentation manages and controls all OpenTelemetry Go
// auto-instrumentation.
type Instrumentation struct {
	target   *process.TargetDetails
	analyzer *process.Analyzer
	manager  *instrumentors.Manager
}

// Error message returned when instrumentation is launched without a valid target
// binary or pid.
var errUndefinedTarget = fmt.Errorf("undefined target Go binary, consider setting the %s environment variable pointing to the target binary to instrument", envTargetExeKey)

func newLogger() logr.Logger {
	zapLog, err := zap.NewProduction()

	var logger logr.Logger
	if err != nil {
		// Fallback to stdr logger.
		logger = stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	} else {
		logger = zapr.NewLogger(zapLog)
	}

	return logger
}

// NewInstrumentation returns a new [Instrumentation] configured with the
// provided opts.
//
// If conflicting or duplicate options are provided, the last one will have
// precedence and be used.
func NewInstrumentation(ctx context.Context, opts ...InstrumentationOption) (*Instrumentation, error) {
	// TODO: pass this in as an option.
	//
	// We likely want to use slog instead of logr in the longterm. Wait until
	// that package has enough Go version support and then switch to that so we
	// can expose it in an option.
	logger := newLogger()
	logger = logger.WithName("Instrumentation")

	c, err := newInstConfig(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}

	pa := process.NewAnalyzer(logger)
	pid, err := pa.DiscoverProcessID(&c.target)
	if err != nil {
		return nil, err
	}

	ctrl, err := opentelemetry.NewController(logger, Version(), c.serviceName, c.traceExp)
	if err != nil {
		return nil, err
	}

	mngr, err := instrumentors.NewManager(logger, ctrl)
	if err != nil {
		return nil, err
	}

	td, err := pa.Analyze(pid, mngr.GetRelevantFuncs())
	if err != nil {
		mngr.Close()
		return nil, err
	}

	allocDetails, err := process.Allocate(logger, pid)
	if err != nil {
		return nil, err
	}
	td.AllocationDetails = allocDetails

	logger.Info(
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
	apply(context.Context, instConfig) (instConfig, error)
}

type instConfig struct {
	traceExp    trace.SpanExporter
	target      process.TargetArgs
	serviceName string
}

func newInstConfig(ctx context.Context, opts []InstrumentationOption) (instConfig, error) {
	var (
		c   instConfig
		err error
	)
	for _, opt := range opts {
		if opt != nil {
			var e error
			c, e = opt.apply(ctx, c)
			err = errors.Join(err, e)
		}
	}

	// Defaults.
	if c.serviceName == "" {
		c.serviceName = c.defualtServiceName()
	}
	if c.traceExp == nil {
		var e error
		c.traceExp, e = opentelemetry.DefaultTraceExporter(ctx, Version())
		err = errors.Join(err, e)
	}

	return c, err
}

func (c instConfig) defualtServiceName() string {
	name := "unknown_service"
	if c.target.ExePath != "" {
		name = fmt.Sprintf("%s:%s", name, filepath.Base(c.target.ExePath))
	}
	return name
}

func (c instConfig) validate() error {
	var zero process.TargetArgs
	if c.target == zero {
		return errUndefinedTarget
	}
	if c.traceExp == nil {
		// We fully control this, so it should never occur. However, the
		// Controller is counting on this part of the code to ensure a nil
		// value is never passed to it so double-check here.
		return errors.New("undefined trace exporter")
	}
	return c.target.Validate()
}

type fnOpt func(context.Context, instConfig) (instConfig, error)

func (o fnOpt) apply(ctx context.Context, c instConfig) (instConfig, error) { return o(ctx, c) }

// WithTarget returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being executed at the provided path.
//
// This option conflicts with [WithPID]. If both are used, the last one
// provided to an [Instrumentation] will be used.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined, this option will conflict with
// [WithEnv]. If both are used, the last one provided to an [Instrumentation]
// will be used.
func WithTarget(path string) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.target = process.TargetArgs{ExePath: path}
		return c, nil
	})
}

// WithServiceName returns an [InstrumentationOption] defining the name of the service running.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_SERVICE_NAME is defined or the service name is defined in
// OTEL_RESOURCE_ATTRIBUTES, this option will conflict with [WithEnv]. If both
// are used, the last one provided to an [Instrumentation] will be used.
func WithServiceName(serviceName string) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.serviceName = serviceName
		return c, nil
	})
}

// WithPID returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being run with the provided PID.
//
// This option conflicts with [WithTarget]. If both are used, the last one
// provided to an [Instrumentation] will be used.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined, this option will conflict with
// [WithEnv]. If both are used, the last one provided to an [Instrumentation]
// will be used.
func WithPID(pid int) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.target = process.TargetArgs{Pid: pid}
		return c, nil
	})
}

var lookupEnv = os.LookupEnv

// WithEnv returns an [InstrumentationOption] that will configure
// [Instrumentation] using the values defined by the following environment
// variables:
//
//   - OTEL_GO_AUTO_TARGET_EXE: sets the target binary
//   - OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES): sets the service name
//
// This option may conflict with [WithTarget], [WithPID], and [WithServiceName]
// if their respective environment variable is defined. If more than one of
// these options are used, the last one provided to an [Instrumentation] will
// be used.
func WithEnv() InstrumentationOption {
	return fnOpt(func(ctx context.Context, c instConfig) (instConfig, error) {
		var err error
		if v, ok := lookupEnv(envTargetExeKey); ok {
			c.target = process.TargetArgs{ExePath: v}
		}
		if _, ok := lookupEnv(envTracesExportersKey); ok {
			// Don't track the lookup value because autoexport does not provide
			// a way to just pass the envoriment value currently. Just use
			// NewSpanExporter which will re-read this value.

			fback, e := opentelemetry.DefaultTraceExporter(ctx, Version())
			err = errors.Join(err, e)
			if e == nil {
				c.traceExp, e = autoexport.NewSpanExporter(
					ctx,
					autoexport.WithFallbackSpanExporter(fback),
				)
				err = errors.Join(err, e)
			}
		}
		if v, ok := lookupServiceName(); ok {
			c.serviceName = v
		}
		return c, err
	})
}

func lookupServiceName() (string, bool) {
	// Prioritize OTEL_SERVICE_NAME over OTEL_RESOURCE_ATTRIBUTES value.
	if v, ok := lookupEnv(envServiceNameKey); ok {
		return v, ok
	}

	v, ok := lookupEnv(envResourceAttrKey)
	if !ok {
		return "", false
	}

	for _, keyval := range strings.Split(strings.TrimSpace(v), ",") {
		key, val, found := strings.Cut(keyval, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == string(semconv.ServiceNameKey) {
			return strings.TrimSpace(val), true
		}
	}

	return "", false
}

// WithTraceExporter return an [InstrumentationOption] that will configure an
// [Instrumentation] to use the provided exp as the OpenTelemetry SpanExporter.
//
// If OTEL_TRACES_EXPORTER is defined, this option will conflict with
// [WithEnv]. If both are used, the last one provided to an [Instrumentation]
// will be used.
func WithTraceExporter(exp trace.SpanExporter) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.traceExp = exp
		return c, nil
	})
}
