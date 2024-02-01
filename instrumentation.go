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
	"runtime"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation"
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
	manager  *instrumentation.Manager
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

	ctrl, err := opentelemetry.NewController(logger, c.tracerProvider(), Version())
	if err != nil {
		return nil, err
	}

	mngr, err := instrumentation.NewManager(logger, ctrl, c.globalImpl)
	if err != nil {
		return nil, err
	}

	td, err := pa.Analyze(pid, mngr.GetRelevantFuncs())
	if err != nil {
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
	mngr.FilterUnusedProbes(td)

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
	return i.manager.Close()
}

// InstrumentationOption applies a configuration option to [Instrumentation].
type InstrumentationOption interface {
	apply(context.Context, instConfig) (instConfig, error)
}

type instConfig struct {
	sampler            trace.Sampler
	traceExp           trace.SpanExporter
	target             process.TargetArgs
	serviceName        string
	additionalResAttrs []attribute.KeyValue
	globalImpl         bool
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
		// This is the OTel recommended default.
		c.traceExp, e = otlptracehttp.New(ctx)
		err = errors.Join(err, e)
	}

	if c.sampler == nil {
		c.sampler = trace.AlwaysSample()
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
		return errors.New("undefined trace exporter")
	}
	return c.target.Validate()
}

func (c instConfig) tracerProvider() *trace.TracerProvider {
	return trace.NewTracerProvider(
		trace.WithSampler(c.sampler),
		trace.WithResource(c.res()),
		trace.WithBatcher(c.traceExp),
		trace.WithIDGenerator(opentelemetry.NewEBPFSourceIDGenerator()),
	)
}

func (c instConfig) res() *resource.Resource {
	runVer := strings.TrimPrefix(runtime.Version(), "go")
	runName := runtime.Compiler
	if runName == "gc" {
		runName = "go"
	}
	runDesc := fmt.Sprintf(
		"go version %s %s/%s",
		runVer, runtime.GOOS, runtime.GOARCH,
	)

	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(c.serviceName),
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetryAutoVersionKey.String(Version()),
		semconv.ProcessRuntimeName(runName),
		semconv.ProcessRuntimeVersion(runVer),
		semconv.ProcessRuntimeDescription(runDesc),
	}

	if len(c.additionalResAttrs) > 0 {
		attrs = append(attrs, c.additionalResAttrs...)
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	)
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
//   - OTEL_TRACES_EXPORTER: sets the trace exporter
//
// This option may conflict with [WithTarget], [WithPID], [WithTraceExporter],
// and [WithServiceName] if their respective environment variable is defined.
// If more than one of these options are used, the last one provided to an
// [Instrumentation] will be used.
//
// The OTEL_TRACES_EXPORTER environment variable value is resolved using the
// [autoexport] package. See that package's documentation for information on
// supported values and registration of custom exporters.
func WithEnv() InstrumentationOption {
	return fnOpt(func(ctx context.Context, c instConfig) (instConfig, error) {
		var err error
		if v, ok := lookupEnv(envTargetExeKey); ok {
			c.target = process.TargetArgs{ExePath: v}
		}
		if _, ok := lookupEnv(envTracesExportersKey); ok {
			// Don't track the lookup value because autoexport does not provide
			// a way to just pass the environment value currently. Just use
			// NewSpanExporter which will re-read this value.

			var e error
			// NewSpanExporter will use an OTLP (HTTP/protobuf) exporter as the
			// default. This is the OTel recommended default.
			c.traceExp, e = autoexport.NewSpanExporter(ctx)
			err = errors.Join(err, e)
		}
		if name, attrs, ok := lookupResourceData(); ok {
			c.serviceName = name
			c.additionalResAttrs = append(c.additionalResAttrs, attrs...)
		}
		return c, err
	})
}

func lookupResourceData() (string, []attribute.KeyValue, bool) {
	// Prioritize OTEL_SERVICE_NAME over OTEL_RESOURCE_ATTRIBUTES value.
	svcName := ""
	if v, ok := lookupEnv(envServiceNameKey); ok {
		svcName = v
	}

	v, ok := lookupEnv(envResourceAttrKey)
	if !ok {
		return svcName, nil, svcName != ""
	}

	var attrs []attribute.KeyValue
	for _, keyval := range strings.Split(strings.TrimSpace(v), ",") {
		key, val, found := strings.Cut(keyval, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == string(semconv.ServiceNameKey) {
			svcName = strings.TrimSpace(val)
		} else {
			attrs = append(attrs, attribute.String(key, strings.TrimSpace(val)))
		}
	}

	if svcName == "" {
		return "", nil, false
	}

	return svcName, attrs, true
}

// WithTraceExporter returns an [InstrumentationOption] that will configure an
// [Instrumentation] to use the provided exp to export OpenTelemetry tracing
// telemetry.
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

// WithSampler returns an [InstrumentationOption] that will configure
// an [Instrumentation] to use the provided sampler to sample OpenTelemetry traces.
func WithSampler(sampler trace.Sampler) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.sampler = sampler
		return c, nil
	})
}

// WithGlobal returns an [InstrumentationOption] that will configure an
// [Instrumentation] to record telemetry from the [OpenTelemetry default global
// implementation]. By default, the OpenTelemetry global implementation is a
// no-op implementation of the OpenTelemetry API. However, by using this
// option, all telemetry that would have been dropped by the global
// implementation will be recorded using telemetry pipelines from the
// configured [Instrumentation].
//
// If the target process overrides the default global implementation (e.g.
// [otel.SetTracerProvider]), the telemetry from that process will go to the
// set implementation. It will not be recorded using the telemetry pipelines
// from the configured [Instrumentation] even if this option is used.
//
// The OpenTelemetry default global implementation is left unchanged (i.e. it
// remains a no-op implementation) if this options is not used.
//
// [OpenTelemetry default global implementation]: https://pkg.go.dev/go.opentelemetry.io/otel
func WithGlobal() InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.globalImpl = true
		return c, nil
	})
}

// WithResourceAttributes returns an [InstrumentationOption] that will configure
// an [Instrumentation] to add the provided attributes to the OpenTelemetry resource.
func WithResourceAttributes(attrs ...attribute.KeyValue) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.additionalResAttrs = append(c.additionalResAttrs, attrs...)
		return c, nil
	})
}
