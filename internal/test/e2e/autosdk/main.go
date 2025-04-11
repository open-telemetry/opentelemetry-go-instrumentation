// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package autosdk is a testing application for the
// [go.opentelemetry.io/auto/sdk] package.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/test/trigger"
	"go.opentelemetry.io/auto/sdk"
)

const (
	pkgName   = "go.opentelemetry.io/auto/internal/test/e2e/autosdk"
	pkgVer    = "v1.23.42"
	schemaURL = "https://some_schema"
)

// Y2K (January 1, 2000).
var y2k = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

type app struct {
	tracer trace.Tracer
}

func (a *app) Run(ctx context.Context, user string, admin bool, in <-chan msg) error {
	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("user", user),
			attribute.Bool("admin", admin),
		),
		trace.WithTimestamp(y2k.Add(500 * time.Microsecond)),
		trace.WithSpanKind(trace.SpanKindServer),
	}
	_, span := a.tracer.Start(ctx, "Run", opts...)
	defer span.End(trace.WithTimestamp(y2k.Add(1 * time.Second)))

	for m := range in {
		span.AddLink(trace.Link{
			SpanContext: m.SpanContext,
			Attributes:  []attribute.KeyValue{attribute.String("data", m.Data)},
		})
	}

	return errors.New("broken")
}

type msg struct {
	SpanContext trace.SpanContext
	Data        string
}

func sig(ctx context.Context) <-chan msg {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(
		pkgName,
		trace.WithInstrumentationVersion(pkgVer),
		trace.WithSchemaURL(schemaURL),
	)

	ts := y2k.Add(10 * time.Microsecond)
	_, span := tracer.Start(ctx, "sig", trace.WithTimestamp(ts))
	defer span.End(trace.WithTimestamp(ts.Add(100 * time.Microsecond)))

	out := make(chan msg, 1)
	out <- msg{SpanContext: span.SpanContext(), Data: "Hello World"}
	close(out)

	return out
}

func main() {
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Wait for auto-instrumentation.
	err := trig.Wait(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	provider := sdk.TracerProvider()
	tracer := provider.Tracer(
		pkgName,
		trace.WithInstrumentationVersion(pkgVer),
		trace.WithSchemaURL(schemaURL),
	)
	app := app{tracer: tracer}

	ctx, span := tracer.Start(ctx, "main", trace.WithTimestamp(y2k))

	err = app.Run(ctx, "Alice", true, sig(ctx))
	if err != nil {
		span.SetStatus(codes.Error, "application error")
		span.RecordError(
			err,
			trace.WithAttributes(attribute.Int("impact", 11)),
			trace.WithTimestamp(y2k.Add(2*time.Second)),
			trace.WithStackTrace(true),
		)
	}

	span.End(trace.WithTimestamp(y2k.Add(5 * time.Second)))
}
