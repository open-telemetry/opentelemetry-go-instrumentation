// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"debug/buildinfo"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"go.opentelemetry.io/auto/pipeline"
)

// Multiplexer supports sending telemetry from multiple resources through the
// same processing and exporting pipeline.
type Multiplexer struct {
	cfg config
}

// NewMultiplexer returns a new *Multiplexer that reuses the provided options as
// a base configuration for all [pipeline.Handler] it creates.
//
// Any exporter provided in the options must support exporting telemetry for
// multiple resources.
func NewMultiplexer(ctx context.Context, options ...Option) (*Multiplexer, error) {
	cfg, err := newConfig(ctx, options)
	if err != nil {
		return nil, err
	}
	return &Multiplexer{cfg: cfg}, nil
}

// Handler returns a new [pipeline.Handler] configured with additional
// resource attributes that follow OpenTelemetry semantic conventions for the
// process associated with the given pid.
//
// If an error occurs while determining process-specific resource attributes,
// the error is logged, and a handler without those attributes is returned.
//
// If Shutdown has already been called on the Multiplexer, the returned handler
// will also be in a shut down state and will not export any telemetry.
func (m Multiplexer) Handler(pid int) *pipeline.Handler {
	c := m.withProcResAttrs(pid)
	return &pipeline.Handler{TraceHandler: newTraceHandler(c)}
}

// Shutdown gracefully shuts down the Multiplexer's span processor.
//
// After Shutdown is called, any subsequent calls to Handler will return a
// handler that is in a shut down state. These handlers will silently drop
// telemetry and will not perform any processing or exporting.
func (m Multiplexer) Shutdown(ctx context.Context) error {
	return m.cfg.spanProcessor.Shutdown(ctx)
}

// withProcResAttrs returns a copy of the Multiplexer's config with additional
// resource attributes based on Go runtime build information for the process
// identified by pid.
//
// It attempts to read the build info of the Go executable at /proc/<pid>/exe
// and extracts semantic convention attributes such as the runtime version and
// compiler name. If this fails, an error is logged and no extra attributes
// are added.
func (m Multiplexer) withProcResAttrs(pid int) (c config) {
	c = m.cfg // Make a shallow copy to modify attributes.

	var attrs []attribute.KeyValue

	path := "/proc/" + strconv.Itoa(pid) + "/exe"
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		c.logger.Error("failed to get Go proc build info", "error", err)
		return c
	}

	// Add Go runtime version as a semantic attribute.
	attrs = append(attrs, semconv.ProcessRuntimeVersion(bi.GoVersion))

	// Try to determine which Go compiler was used.
	var compiler string
	for _, setting := range bi.Settings {
		if setting.Key == "-compiler" {
			compiler = setting.Value
			break
		}
	}
	switch compiler {
	case "":
		c.logger.Debug("failed to identify Go compiler")
	case "gc":
		attrs = append(attrs, semconv.ProcessRuntimeName("go"))
	default:
		attrs = append(attrs, semconv.ProcessRuntimeName(compiler))
	}

	// Prepend process-specific attributes so user-provided ones have priority.
	c.resAttrs = append(attrs, c.resAttrs...)

	return c
}
