// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/sdk"
)

const pkgName = "go.opentelemetry.io/auto/internal/test/e2e/autosdk"

// Y2K (January 1, 2000).
var y2k = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

type app struct {
	tracer trace.Tracer
}

func (a *app) Run(ctx context.Context, user string, admin bool) error {
	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("user", user),
			attribute.Bool("admin", admin),
		),
		trace.WithTimestamp(y2k.Add(500 * time.Microsecond)),
		trace.WithSpanKind(trace.SpanKindInternal),
	}
	_, span := a.tracer.Start(ctx, "Run", opts...)
	defer span.End(trace.WithTimestamp(y2k.Add(1 * time.Second)))

	return errors.New("broken")
}

func main() {
	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	provider := sdk.GetTracerProvider()
	tracer := provider.Tracer(
		pkgName,
		trace.WithInstrumentationVersion("v1.23.42"),
		trace.WithSchemaURL("https://some_schema"),
	)
	app := app{tracer: tracer}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx, span := tracer.Start(ctx, "main", trace.WithTimestamp(y2k))
	defer span.End(trace.WithTimestamp(y2k.Add(5 * time.Second)))

	err := app.Run(ctx, "Alice", true)
	if err != nil {
		span.SetStatus(codes.Error, "application error")
		span.RecordError(
			err,
			trace.WithAttributes(attribute.Int("impact", 11)),
			trace.WithTimestamp(y2k.Add(2*time.Second)),
			trace.WithStackTrace(true),
		)
	}

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
