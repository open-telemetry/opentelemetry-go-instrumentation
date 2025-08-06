// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/contrib/detectors/autodetect"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation"
)

const (
	// envServiceName is the key for the envoriment variable value containing
	// the service name.
	envServiceNameKey = "OTEL_SERVICE_NAME"
	// envResourceAttrKey is the key for the environment variable value
	// containing OpenTelemetry Resource attributes.
	envResourceAttrKey = "OTEL_RESOURCE_ATTRIBUTES"
	// envResourceDetectorsKey is the key for the environment variable value
	// containing comma-separated list of resource detector IDs to enable.
	envResourceDetectorsKey = "OTEL_GO_AUTO_RESOURCE_DETECTORS"
	// envLogLevelKey is the key for the environment variable value containing
	// the log level.
	envLogLevelKey = "OTEL_LOG_LEVEL"
)

// Option configures a [traceHandler] via [NewHandler].
type Option interface {
	apply(context.Context, config) (config, error)
}

type fnOpt func(context.Context, config) (config, error)

func (o fnOpt) apply(ctx context.Context, c config) (config, error) {
	return o(ctx, c)
}

// WithServiceName returns an [Option] defining the name of the service running.
//
// If multiple of these options are provided, the last one will be used.
//
// If OTEL_SERVICE_NAME is defined or the service name is defined in
// OTEL_RESOURCE_ATTRIBUTES, this option will conflict with [WithEnv]. If both
// are used, the last one provided will be used.
func WithServiceName(name string) Option {
	return fnOpt(func(_ context.Context, c config) (config, error) {
		c.resAttrs = append(c.resAttrs, semconv.ServiceName(name))
		return c, nil
	})
}

// WithLogger returns an [Option] that will configure logger used.
//
// If this option and [WithEnv] are used, OTEL_LOG_LEVEL is ignored. This
// passed logger takes precedence and is used as-is.
//
// If this option is not used, an [slog.Loogger] backed by an
// [slog.JSONHandler] outputting to STDERR as a default.
func WithLogger(l *slog.Logger) Option {
	return fnOpt(func(_ context.Context, c config) (config, error) {
		c.logger = l
		return c, nil
	})
}

// WithResourceAttributes returns an [Option] that will configure attributes to
// be added to the OpenTelemetry Resource.
func WithResourceAttributes(attrs ...attribute.KeyValue) Option {
	return fnOpt(func(_ context.Context, c config) (config, error) {
		c.resAttrs = append(c.resAttrs, attrs...)
		return c, nil
	})
}

// WithResourceDetector returns an [Option] that will configure a resource
// detector to use when resolving a [resource.Resource].
//
// Multiple WithResourceDetector options can be provided and all detected
// resources will be merged together along with resources from WithEnv and
// other configuration options.
func WithResourceDetector(detector resource.Detector) Option {
	return fnOpt(func(ctx context.Context, c config) (config, error) {
		detectedRes, err := detector.Detect(ctx)
		if err != nil {
			return c, fmt.Errorf("failed to detect resource: %w", err)
		}

		c.detectorResources = append(c.detectorResources, detectedRes)
		return c, nil
	})
}

// WithTraceExporter returns an [Option] that will configure exp as the
// OpenTelemetry tracing exporter used.
//
// If OTEL_TRACES_EXPORTER is defined, this option will conflict with
// [WithEnv]. If both are used, the last one provided will be used.
func WithTraceExporter(exp sdk.SpanExporter) Option {
	return fnOpt(func(_ context.Context, c config) (config, error) {
		c.exporter = exp
		return c, nil
	})
}

var (
	lookupEnv = os.LookupEnv
	getEnv    = os.Getenv
)

// WithEnv returns an [Option] that will apply configuration using the values
// defined by the following environment variables:
//
//   - OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES): sets the service name
//   - OTEL_GO_AUTO_RESOURCE_DETECTORS: sets the resource detectors to enable
//   - OTEL_TRACES_EXPORTER: sets the trace exporter
//   - OTEL_LOG_LEVEL: sets the default logger's minimum logging level
//
// This option will conflict with [WithTraceExporter] and [WithServiceName].
// The last [Option] provided will be used.
//
// Resources detected from OTEL_GO_AUTO_RESOURCE_DETECTORS will be merged with
// resources from any [WithResourceDetector] options provided.
//
// If [WithLogger] is used, OTEL_LOG_LEVEL will not be used. Instead, the
// [slog.Logger] passed to that option will be used as-is.
//
// If [WithLogger] is not used, OTEL_LOG_LEVEL will be parsed and the default
// logger will use that level as its minimum logging level.
//
// The OTEL_TRACES_EXPORTER environment variable value is resolved using the
// [autoexport] package. See that package's documentation for information on
// supported values and registration of custom exporters.
//
// The OTEL_GO_AUTO_RESOURCE_DETECTORS environment variable value should be a
// comma-separated list of resource detector IDs registered with
// [autodetect.Register]. See the [autodetect] package for details. If not set,
// no additional resource detectors will be enabled.
//
// If OTEL_RESOURCE_ATTRIBUTES is defined, it will be used to merge attributes
// into any attributes defined by OTEL_RESOURCE_ATTRIBUTES.
func WithEnv() Option {
	return fnOpt(func(ctx context.Context, c config) (config, error) {
		var err error
		// NewSpanExporter will use an OTLP (HTTP/protobuf) exporter as the
		// default. This is the OTel recommended default.
		c.exporter, err = autoexport.NewSpanExporter(ctx)

		c.resAttrs = append(c.resAttrs, lookupResourceData()...)

		r, e := lookupDetectors(ctx)
		err = errors.Join(err, e)
		if r != nil {
			c.detectorResources = append(c.detectorResources, r)
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
		return c, err
	})
}

func lookupResourceData() []attribute.KeyValue {
	rawVal := getEnv(envResourceAttrKey)
	pairs := strings.Split(strings.TrimSpace(rawVal), ",")

	var attrs []attribute.KeyValue
	for _, pair := range pairs {
		key, val, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		attrs = append(attrs, attribute.String(key, val))
	}

	if v, ok := lookupEnv(envServiceNameKey); ok {
		attrs = append(attrs, semconv.ServiceName(v))
	}

	return attrs
}

// lookupDetectors parses resource detectors from environment variable and
// returns the detected resource.
func lookupDetectors(ctx context.Context) (*resource.Resource, error) {
	detectorsStr := getEnv(envResourceDetectorsKey)
	detectorsStr = strings.TrimSpace(detectorsStr)
	if detectorsStr == "" {
		return nil, nil // No detectors configured
	}

	detectors := strings.Split(detectorsStr, ",")

	ids := make([]autodetect.ID, 0, len(detectors))
	for _, d := range detectors {
		d = strings.TrimSpace(d)
		ids = append(ids, autodetect.ID(d))
	}

	detector, err := autodetect.Detector(ids...)
	if err != nil {
		return nil, fmt.Errorf("create autodetect detector: %w", err)
	}

	return detector.Detect(ctx)
}

// newLogger is used for testing.
var newLogger = newLoggerFunc

func newLoggerFunc(level slog.Leveler) *slog.Logger {
	opts := &slog.HandlerOptions{AddSource: true, Level: level}
	h := slog.NewJSONHandler(os.Stderr, opts)
	return slog.New(h)
}

type config struct {
	logger            *slog.Logger
	exporter          sdk.SpanExporter
	resAttrs          []attribute.KeyValue
	detectorResources []*resource.Resource

	spanProcessor sdk.SpanProcessor
	idGenerator   *idGenerator
}

func newConfig(ctx context.Context, options []Option) (config, error) {
	c := config{
		resAttrs: []attribute.KeyValue{
			semconv.ServiceName(defaultServiceName()),
		},
		idGenerator: newIDGenerator(),
	}

	var err error
	for _, opt := range options {
		var e error
		c, e = opt.apply(ctx, c)
		err = errors.Join(err, e)
	}

	if c.exporter == nil {
		var e error
		c.exporter, e = otlptracehttp.New(ctx)
		if e != nil {
			err = errors.Join(err, e)
		}
	}
	c.spanProcessor = sdk.NewBatchSpanProcessor(c.exporter)

	return c, err
}

func defaultServiceName() string {
	executable, err := os.Executable()
	if err != nil {
		return "unknown_service:go"
	}
	return "unknown_service:" + filepath.Base(executable)
}

func (c config) Logger() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}
	return newLogger(nil)
}

func (c config) TracerProvider() *sdk.TracerProvider {
	return sdk.NewTracerProvider(
		// Sample everything. The actual sampling is done in the eBPF probes
		// before it reaches this tracerProvider.
		sdk.WithSampler(sdk.AlwaysSample()),
		sdk.WithResource(c.resource()),
		sdk.WithSpanProcessor(c.spanProcessor),
		sdk.WithIDGenerator(c.idGenerator),
	)
}

func (c config) resource() *resource.Resource {
	r := c.baseResource()

	for i, detectorRes := range c.detectorResources {
		var err error
		r, err = resource.Merge(r, detectorRes)
		if err != nil {
			// Most likely a schema URL conflict, which means that the detector
			// returned a resource with a schema URL that conflicts with the
			// one already in the resource.
			//
			// This is not a fatal error, so we log it and continue merging the
			// resources. The final resource will still contain the attributes
			// from the detector, but the schema URL will be empty.
			c.Logger().Error(
				"failed to merge detector resource",
				"error", err,
				"detector", i,
			)
		}
	}

	return r
}

func (c config) baseResource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		append(
			[]attribute.KeyValue{
				semconv.TelemetrySDKLanguageGo,
				semconv.TelemetryDistroVersion(instrumentation.Version),
				semconv.TelemetryDistroName(instrumentation.Name),
			},
			c.resAttrs...,
		)...,
	)
}
