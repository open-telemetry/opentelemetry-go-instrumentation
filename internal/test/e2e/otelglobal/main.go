// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

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
	// registering unused tracers to test how we handle a non-trivial tracers map
	setUnusedTracers()
	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	createMainSpan(context.Background())

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
