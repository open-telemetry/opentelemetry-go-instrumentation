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

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	// envServiceName is the key for the envoriment variable value containing
	// the service name.
	envServiceNameKey = "OTEL_SERVICE_NAME"
	// envResourceAttrKey is the key for the environment variable value
	// containing OpenTelemetry Resource attributes.
	envResourceAttrKey = "OTEL_RESOURCE_ATTRIBUTES"
	// envTracesExportersKey is the key for the environment variable value
	// containing what OpenTelemetry trace exporter to use.
	envTracesExportersKey = "OTEL_TRACES_EXPORTER"
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
//   - OTEL_TRACES_EXPORTER: sets the trace exporter
//   - OTEL_LOG_LEVEL: sets the default logger's minimum logging level
//
// This option will conflict with [WithTraceExporter] and [WithServiceName].
// The last [Option] provided will be used.
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
func WithEnv() Option {
	return fnOpt(func(ctx context.Context, c config) (config, error) {
		var err error
		if _, ok := lookupEnv(envTracesExportersKey); ok {
			// Don't track the lookup value because autoexport does not provide
			// a way to just pass the environment value currently. Just use
			// NewSpanExporter which will re-read this value.

			var e error
			// NewSpanExporter will use an OTLP (HTTP/protobuf) exporter as the
			// default. This is the OTel recommended default.
			c.exporter, e = autoexport.NewSpanExporter(ctx)
			err = errors.Join(err, e)
		}

		c.resAttrs = append(c.resAttrs, lookupResourceData()...)

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

// newLogger is used for testing.
var newLogger = newLoggerFunc

func newLoggerFunc(level slog.Leveler) *slog.Logger {
	opts := &slog.HandlerOptions{AddSource: true, Level: level}
	h := slog.NewJSONHandler(os.Stderr, opts)
	return slog.New(h)
}

type config struct {
	logger   *slog.Logger
	exporter sdk.SpanExporter
	resAttrs []attribute.KeyValue
}

func newConfig(ctx context.Context, options []Option) (config, error) {
	c := config{
		resAttrs: []attribute.KeyValue{
			semconv.ServiceName(defaultServiceName()),
		},
	}

	var err error
	for _, opt := range options {
		var e error
		c, e = opt.apply(ctx, c)
		err = errors.Join(err, e)
	}

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

func (c config) TracerProvider(ctx context.Context) (*sdk.TracerProvider, error) {
	exp := c.exporter
	if exp == nil {
		var err error
		exp, err = otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
	}

	return sdk.NewTracerProvider(
		// Sample everything. The actual sampling is done in the eBPF probes
		// before it reaches this tracerProvider.
		sdk.WithSampler(sdk.AlwaysSample()),
		sdk.WithResource(c.resource()),
		sdk.WithBatcher(exp),
		sdk.WithIDGenerator(newIDGenerator()),
	), nil
}

func (c config) resource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		append(
			[]attribute.KeyValue{
				semconv.TelemetrySDKLanguageGo,
				semconv.TelemetryDistroNameKey.String("opentelemetry-go-instrumentation"),
			},
			c.resAttrs...,
		)...,
	)
}
