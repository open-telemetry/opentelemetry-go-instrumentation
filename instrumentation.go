// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

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
	// envOtelGlobalImplKey is the key for the environment variable value enabling to opt-in for the
	// OpenTelemetry global implementation. It should be a boolean value.
	envOtelGlobalImplKey = "OTEL_GO_AUTO_GLOBAL"
	// envLogLevelKey is the key for the environment variable value containing the log level.
	envLogLevelKey = "OTEL_LOG_LEVEL"
)

// Instrumentation manages and controls all OpenTelemetry Go
// auto-instrumentation.
type Instrumentation struct {
	target   *process.Info
	analyzer *process.Analyzer
	manager  *instrumentation.Manager

	stopMu  sync.Mutex
	stop    context.CancelFunc
	stopped chan struct{}
}

// Error message returned when instrumentation is launched without a valid target
// binary or pid.
var errUndefinedTarget = fmt.Errorf("undefined target Go binary, consider setting the %s environment variable pointing to the target binary to instrument", envTargetExeKey)

// NewInstrumentation returns a new [Instrumentation] configured with the
// provided opts.
//
// If conflicting or duplicate options are provided, the last one will have
// precedence and be used.
func NewInstrumentation(ctx context.Context, opts ...InstrumentationOption) (*Instrumentation, error) {
	c, err := newInstConfig(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}

	pa := process.NewAnalyzer(c.logger)
	pid, err := pa.DiscoverProcessID(ctx, &c.target)
	if err != nil {
		return nil, err
	}

	err = pa.SetBuildInfo(pid)
	if err != nil {
		return nil, err
	}

	ctrl, err := opentelemetry.NewController(c.logger, c.tracerProvider(pa.BuildInfo))
	if err != nil {
		return nil, err
	}

	cp := convertConfigProvider(c.cp)
	mngr, err := instrumentation.NewManager(c.logger, ctrl, c.globalImpl, cp, Version())
	if err != nil {
		return nil, err
	}

	td, err := pa.Analyze(pid, mngr.GetRelevantFuncs())
	if err != nil {
		return nil, err
	}

	alloc, err := process.Allocate(c.logger, pid)
	if err != nil {
		return nil, err
	}
	td.Allocation = alloc

	c.logger.Info(
		"target process analysis completed",
		"pid", td.PID,
		"go_version", td.GoVersion,
		"dependencies", td.Modules,
		"total_functions_found", len(td.Functions),
	)
	mngr.FilterUnusedProbes(td)

	return &Instrumentation{
		target:   td,
		analyzer: pa,
		manager:  mngr,
	}, nil
}

// Load loads and attaches the relevant probes to the target process.
func (i *Instrumentation) Load(ctx context.Context) error {
	return i.manager.Load(ctx, i.target)
}

// Run starts the instrumentation. It must be called after [Instrumentation.Load].
//
// This function will not return until either ctx is done, an unrecoverable
// error is encountered, or Close is called.
func (i *Instrumentation) Run(ctx context.Context) error {
	ctx, err := i.newStop(ctx)
	if err != nil {
		return err
	}

	err = i.manager.Run(ctx)
	close(i.stopped)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

func (i *Instrumentation) newStop(parent context.Context) (context.Context, error) {
	i.stopMu.Lock()
	defer i.stopMu.Unlock()

	if i.stop != nil {
		return parent, errors.New("instrumentation already running")
	}

	ctx, stop := context.WithCancel(parent)
	i.stop, i.stopped = stop, make(chan struct{})
	return ctx, nil
}

// Close closes the Instrumentation, cleaning up all used resources.
func (i *Instrumentation) Close() error {
	i.stopMu.Lock()
	defer i.stopMu.Unlock()

	if i.stop == nil {
		// if stop is not set, the instrumentation is not running
		// stop the manager to clean up resources
		return i.manager.Stop()
	}

	i.stop()
	<-i.stopped
	i.stop, i.stopped = nil, nil

	return nil
}

// InstrumentationOption applies a configuration option to [Instrumentation].
type InstrumentationOption interface {
	apply(context.Context, instConfig) (instConfig, error)
}

type instConfig struct {
	traceExp           trace.SpanExporter
	target             process.TargetArgs
	serviceName        string
	additionalResAttrs []attribute.KeyValue
	globalImpl         bool
	logger             *slog.Logger
	sampler            Sampler
	cp                 ConfigProvider
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
		c.serviceName = c.defaultServiceName()
	}
	if c.traceExp == nil {
		var e error
		// This is the OTel recommended default.
		c.traceExp, e = otlptracehttp.New(ctx)
		err = errors.Join(err, e)
	}

	if c.sampler == nil {
		c.sampler = DefaultSampler()
	}

	if c.logger == nil {
		c.logger = newLogger(nil)
	}

	if c.cp == nil {
		c.cp = newNoopConfigProvider(c.sampler)
	}

	return c, err
}

func (c instConfig) defaultServiceName() string {
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

func (c instConfig) tracerProvider(bi *buildinfo.BuildInfo) *trace.TracerProvider {
	return trace.NewTracerProvider(
		// the actual sampling is done in the eBPF probes.
		// this is just to make sure that we export all spans we get from the probes
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(c.res(bi)),
		trace.WithBatcher(c.traceExp),
		trace.WithIDGenerator(opentelemetry.NewEBPFSourceIDGenerator()),
	)
}

func (c instConfig) res(bi *buildinfo.BuildInfo) *resource.Resource {
	runVer := bi.GoVersion

	var compiler string

	for _, setting := range bi.Settings {
		if setting.Key == "-compiler" {
			compiler = setting.Value
			break
		}
	}

	runName := compiler
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
		semconv.TelemetryDistroVersionKey.String(Version()),
		semconv.TelemetryDistroNameKey.String("opentelemetry-go-instrumentation"),
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

// newLogger is used for testing.
var newLogger = newLoggerFunc

func newLoggerFunc(level slog.Leveler) *slog.Logger {
	opts := &slog.HandlerOptions{AddSource: true, Level: level}
	h := slog.NewJSONHandler(os.Stderr, opts)
	return slog.New(h)
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
//   - OTEL_GO_AUTO_GLOBAL: enables the OpenTelemetry global implementation
//   - OTEL_LOG_LEVEL: sets the default logger's minimum logging level
//   - OTEL_TRACES_SAMPLER: sets the trace sampler
//   - OTEL_TRACES_SAMPLER_ARG: optionally sets the trace sampler argument
//
// This option may conflict with [WithTarget], [WithPID], [WithTraceExporter],
// [WithServiceName], [WithGlobal], and [WithSampler] if their respective environment variable is defined.
// If more than one of these options are used, the last one provided to an
// [Instrumentation] will be used.
//
// If [WithLogger] is used, OTEL_LOG_LEVEL will not be used for the
// [Instrumentation] logger. Instead, the [slog.Logger] passed to that option
// will be used as-is.
//
// If [WithLogger] is not used, OTEL_LOG_LEVEL will be parsed and the default
// logger used by the configured [Instrumentation] will use that level as its
// minimum logging level.
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

		var e error
		// NewSpanExporter will use an OTLP (HTTP/protobuf) exporter as the
		// default, unless OTLP_TRACES_EXPORTER is set. This is the OTel
		// recommended default.
		c.traceExp, e = autoexport.NewSpanExporter(ctx)
		err = errors.Join(err, e)

		if name, attrs, ok := lookupResourceData(); ok {
			c.serviceName = name
			c.additionalResAttrs = append(c.additionalResAttrs, attrs...)
		}
		if val, ok := lookupEnv(envOtelGlobalImplKey); ok {
			boolVal, err := strconv.ParseBool(val)
			if err == nil {
				c.globalImpl = boolVal
			}
		}
		if val, ok := lookupEnv(envLogLevelKey); c.logger == nil && ok {
			var level slog.Level
			if e := level.UnmarshalText([]byte(val)); e != nil {
				e = fmt.Errorf("parse log level %q: %w", val, e)
				err = errors.Join(err, e)
			} else {
				c.logger = newLogger(level)
			}
		}
		if s, e := newSamplerFromEnv(lookupEnv); e != nil {
			err = errors.Join(err, e)
		} else {
			c.sampler = s
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
//
// This currently is a no-op. It is expected to take effect in the next release.
func WithSampler(sampler Sampler) InstrumentationOption {
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
// If OTEL_GO_AUTO_GLOBAL is defined, this option will conflict with
// [WithEnv]. If both are used, the last one provided to an [Instrumentation]
// will be used.
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

// WithLogger returns an [InstrumentationOption] that will configure an
// [Instrumentation] to use the provided logger.
//
// If this option is used and [WithEnv] is also used, OTEL_LOG_LEVEL is ignored
// by the configured [Instrumentation]. This passed logger takes precedence and
// is used as-is.
//
// If this option is not used, the [Instrumentation] will use an [slog.Loogger]
// backed by an [slog.JSONHandler] outputting to STDERR as a default.
func WithLogger(logger *slog.Logger) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.logger = logger
		return c, nil
	})
}

// WithConfigProvider returns an [InstrumentationOption] that will configure
// an [Instrumentation] to use the provided ConfigProvider. The ConfigProvider
// is used to provide the initial configuration and update the configuration of
// the instrumentation in runtime.
func WithConfigProvider(cp ConfigProvider) InstrumentationOption {
	return fnOpt(func(_ context.Context, c instConfig) (instConfig, error) {
		c.cp = cp
		return c, nil
	})
}
