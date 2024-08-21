// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
)

// Controller handles OpenTelemetry telemetry generation for events.
type Controller struct {
	logger         logr.Logger
	version        string
	tracerProvider trace.TracerProvider
	tracersMap     map[tracerID]trace.Tracer
	bootTime       int64
}

type tracerID struct{ name, version, schema string }

func (c *Controller) getTracer(pkg, tracerName, version, schema string) trace.Tracer {
	// Default Tracer ID, if the user does not provide one.
	tID := tracerID{name: pkg, version: c.version}
	if tracerName != "" {
		tID = tracerID{name: tracerName, version: version, schema: schema}
	}

	t, exists := c.tracersMap[tID]
	if exists {
		return t
	}

	var newTracer trace.Tracer
	if tracerName != "" {
		// If the user has provided a tracer, use it.
		newTracer = c.tracerProvider.Tracer(tracerName, trace.WithInstrumentationVersion(version), trace.WithSchemaURL(schema))
	} else {
		newTracer = c.tracerProvider.Tracer(
			"go.opentelemetry.io/auto/"+pkg,
			trace.WithInstrumentationVersion(c.version),
			trace.WithSchemaURL(schema),
		)
	}

	c.tracersMap[tID] = newTracer
	return newTracer
}

// Trace creates a trace span for event.
func (c *Controller) Trace(event *probe.Event) {
	for _, se := range event.SpanEvents {
		c.logger.V(1).Info("got event", "kind", event.Kind.String(), "pkg", event.Package, "attrs", se.Attributes, "traceID", se.SpanContext.TraceID().String(), "spanID", se.SpanContext.SpanID().String())
		ctx := context.Background()

		if se.SpanContext == nil {
			c.logger.V(1).Info("got event without context - dropping")
			return
		}

		// TODO: handle remote parent
		if se.ParentSpanContext != nil {
			ctx = trace.ContextWithSpanContext(ctx, *se.ParentSpanContext)
		}

		ctx = ContextWithEBPFEvent(ctx, *se)
		c.logger.V(1).Info("getting tracer", "name", se.TracerName, "version", se.TracerVersion, "schema", se.TracerSchema)
		_, span := c.getTracer(event.Package, se.TracerName, se.TracerVersion, se.TracerSchema).
			Start(ctx, se.SpanName,
				trace.WithAttributes(se.Attributes...),
				trace.WithSpanKind(event.Kind),
				trace.WithTimestamp(c.convertTime(se.StartTime)))
		span.SetStatus(se.Status.Code, se.Status.Description)
		span.End(trace.WithTimestamp(c.convertTime(se.EndTime)))
	}
}

func (c *Controller) convertTime(t int64) time.Time {
	return time.Unix(0, c.bootTime+t)
}

// NewController returns a new initialized [Controller].
func NewController(logger logr.Logger, tracerProvider trace.TracerProvider, ver string) (*Controller, error) {
	logger = logger.WithName("Controller")

	bt, err := utils.EstimateBootTimeOffset()
	if err != nil {
		return nil, err
	}

	return &Controller{
		logger:         logger,
		version:        ver,
		tracerProvider: tracerProvider,
		tracersMap:     make(map[tracerID]trace.Tracer),
		bootTime:       bt,
	}, nil
}

// Shutdown shuts down the OpenTelemetry TracerProvider.
//
// Once shut down, calls to Trace will result in no-op spans (i.e. dropped).
func (c *Controller) Shutdown(ctx context.Context) error {
	if s, ok := c.tracerProvider.(interface {
		Shutdown(context.Context) error
	}); ok {
		// Default TracerProvider implementation.
		return s.Shutdown(ctx)
	}
	return nil
}
