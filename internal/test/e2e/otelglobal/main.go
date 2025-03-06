// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/test/trigger"
)

const (
	name    = "trace-example"
	version = "v1.23.42"
	schema  = "https://some_schema"
)

var tracer = otel.Tracer(
	name,
	trace.WithInstrumentationVersion(version),
	trace.WithSchemaURL(schema),
)

func setUnusedTracers() {
	for i := range 20 {
		otel.Tracer(fmt.Sprintf("unused%d", i))
	}
}

func innerFunction(ctx context.Context) {
	t := trace.SpanFromContext(ctx).TracerProvider().Tracer(
		name,
		trace.WithInstrumentationVersion(version),
		trace.WithSchemaURL(schema),
	)

	_, span := t.Start(ctx, "child")
	defer span.End()

	span.SetAttributes(attribute.String("inner.key", "inner.value"))
	span.SetAttributes(attribute.Bool("cat.on_keyboard", true))
	span.SetName("child override")

	err := errors.New("i deleted the prod db sry")
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	span.AddLink(trace.Link{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{0x2},
			SpanID:     trace.SpanID{0x1},
			TraceFlags: trace.FlagsSampled,
		}),
	})
}

func createMainSpan(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "parent", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	innerFunction(ctx)

	intAttr := attribute.Int("int_key", 42)
	strAttr := attribute.String("string_key", "forty-two")
	boolAttr := attribute.Bool("bool_key", true)
	floatAttr := attribute.Float64("float_key", 42.3)
	span.SetAttributes(intAttr, strAttr, boolAttr, floatAttr)
	span.SetStatus(codes.Ok, "this msg won't be seen")
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

	// registering unused tracers to test how we handle a non-trivial tracers map
	setUnusedTracers()

	createMainSpan(ctx)
}
