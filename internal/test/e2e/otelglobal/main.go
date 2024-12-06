// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"go.opentelemetry.io/auto/internal/test/trigger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer(
	"trace-example",
	trace.WithInstrumentationVersion("v1.23.42"),
	trace.WithSchemaURL("https://some_schema"),
)

func setUnusedTracers() {
	for i := range 20 {
		otel.Tracer(fmt.Sprintf("unused%d", i))
	}
}

func innerFunction(ctx context.Context) {
	_, span := tracer.Start(ctx, "child")
	defer span.End()

	span.SetAttributes(attribute.String("inner.key", "inner.value"))
	span.SetAttributes(attribute.Bool("cat.on_keyboard", true))
	span.SetName("child override")
	span.SetStatus(codes.Error, "i deleted the prod db sry")
}

func createMainSpan(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "parent")
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
