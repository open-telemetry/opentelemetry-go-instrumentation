package otelsdk

import (
	"context"
	"debug/buildinfo"
	"strconv"

	"go.opentelemetry.io/auto/export"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Multiplexer supports sending telemetry from multiple resources through the
// same processing and exporting pipeline.
type Multiplexer struct {
	cfg config
}

// NewMultiplexer returns a new *Multiplexer that will reuse the passed options
// as a base configuration for all [export.Handler] it returns.
//
// Any exporter passed in an option needs to support exporting telemetry from
// multiple resources.
func NewMultiplexer(ctx context.Context, options ...Option) (*Multiplexer, error) {
	cfg, err := newConfig(ctx, options)
	if err != nil {
		return nil, err
	}
	return &Multiplexer{cfg: cfg}, nil
}

// Handler returns a new [export.Handler] configured with additional resources
// attributes conforming to OpenTelemetry semantic conventions for the process
// associated with pid. The returned [export.Handler] will use the same
// processing and export pipeline the Multiplexer is configured with.
//
// If an error occurs determining process resource attributes, that error is
// logged to the configured Multiplexer logger and the default resource that
// does not include process attributes is used instead.
func (m Multiplexer) Handler(pid int) *export.Handler {
	return newHandler(m.withProcResAttrs(pid))
}

// withProcResAttrs returns a copy of the Multiplexer config with OTel
// semantic convention attributes for a process added to the configs resAttrs.
func (m Multiplexer) withProcResAttrs(pid int) (c config) {
	c = m.cfg // Make a copy.

	var attrs []attribute.KeyValue

	path := "/proc/" + strconv.Itoa(pid) + "/exe"
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		c.logger.Error("failed to get Go proc build info", "error", err)
		return c
	}

	attrs = append(attrs, semconv.ProcessRuntimeVersion(bi.GoVersion))

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

	c.resAttrs = append(attrs, c.resAttrs...) // User attrs have highest priority.

	return c
}

func (m Multiplexer) Shutdown(ctx context.Context) error {
	return m.cfg.spanProcessor.Shutdown(ctx)
}
