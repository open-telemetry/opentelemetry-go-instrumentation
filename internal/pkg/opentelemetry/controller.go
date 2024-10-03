// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

// Controller handles OpenTelemetry telemetry generation for events.
type Controller struct {
	logger         *slog.Logger
	version        string
	tracerProvider trace.TracerProvider
	tracersMap     map[tracerID]trace.Tracer
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
		c.logger.Debug("got event", "kind", event.Kind.String(), "pkg", event.Package, "attrs", se.Attributes, "traceID", se.SpanContext.TraceID().String(), "spanID", se.SpanContext.SpanID().String())
		ctx := context.Background()

		if se.SpanContext == nil {
			c.logger.Debug("got event without context - dropping")
			return
		}

		// TODO: handle remote parent
		if se.ParentSpanContext != nil {
			ctx = trace.ContextWithSpanContext(ctx, *se.ParentSpanContext)
		}

		kind := se.Kind
		if kind == trace.SpanKindUnspecified {
			kind = event.Kind
		}

		ctx = ContextWithEBPFEvent(ctx, *se)
		c.logger.Debug("getting tracer", "name", se.TracerName, "version", se.TracerVersion, "schema", se.TracerSchema)
		_, span := c.getTracer(event.Package, se.TracerName, se.TracerVersion, se.TracerSchema).
			Start(ctx, se.SpanName,
				trace.WithAttributes(se.Attributes...),
				trace.WithSpanKind(kind),
				trace.WithTimestamp(se.StartTime),
				trace.WithLinks(se.Links...),
			)
		for name, opts := range se.Events {
			span.AddEvent(name, opts...)
		}
		span.SetStatus(se.Status.Code, se.Status.Description)
		span.End(trace.WithTimestamp(se.EndTime))
	}
}

// NewController returns a new initialized [Controller].
func NewController(logger *slog.Logger, tracerProvider trace.TracerProvider, ver string) (*Controller, error) {
	return &Controller{
		logger:         logger,
		version:        ver,
		tracerProvider: tracerProvider,
		tracersMap:     make(map[tracerID]trace.Tracer),
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
