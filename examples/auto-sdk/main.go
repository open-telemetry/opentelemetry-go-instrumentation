// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const service = "go.opentelemetry.io/auto/examples/auto-sdk"

func main() {
	otel.SetTracerProvider(auto.TracerProvider())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	run(ctx)
}

func run(ctx context.Context) {
	for i := 0; ; i++ {
		outter(ctx, i)
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func outter(ctx context.Context, i int) {
	fmt.Printf("outter-%d...", i)

	tracer := otel.Tracer(service)

	var span trace.Span
	ctx, span = tracer.Start(
		ctx,
		"outter",
		trace.WithAttributes(attribute.Int("i", i)),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	s := time.Duration(3 + rand.Intn(2))
	wait(ctx, s*time.Second)

	fmt.Println("done")
}

func wait(ctx context.Context, d time.Duration) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(service)
	var span trace.Span
	ctx, span = tracer.Start(
		ctx,
		"wait",
		trace.WithAttributes(attribute.Int64("duration", int64(d))),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	defer span.End()

	c := doTick(ctx, d)

	select {
	case err := <-c:
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "timeout")
		}
	}
}

func doTick(ctx context.Context, d time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() {
		tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(service)
		_, span := tracer.Start(
			context.Background(), // Async, not a child.
			"doTick",
			trace.WithLinks(trace.Link{
				SpanContext: trace.SpanContextFromContext(ctx),
				Attributes: []attribute.KeyValue{
					attribute.Bool("spawner", true),
				},
			}),
			trace.WithSpanKind(trace.SpanKindProducer),
		)
		defer span.End()

		t := time.NewTicker(d)
		defer t.Stop()

		select {
		case <-ctx.Done():
		case <-t.C:
		}
		done <- ctx.Err()
		close(done)
	}()
	return done
}
