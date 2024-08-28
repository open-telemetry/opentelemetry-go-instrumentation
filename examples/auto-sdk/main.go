// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/auto"
)

func main() {
	tracer := auto.TracerProvider().Tracer("go.opentelemetry.io/auto/examples/auto-sdk")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	//sc0 := trace.NewSpanContext(trace.SpanContextConfig{
	//	TraceID:    trace.TraceID{0x01},
	//	SpanID:     trace.SpanID{0x01},
	//	TraceFlags: trace.FlagsSampled,
	//})

	for i := 0; ; i++ {
		name := fmt.Sprintf("span-%d", i)
		fmt.Printf("%s...", name)

		_, span := tracer.Start(
			ctx,
			name,
			//trace.WithAttributes(attribute.Int("i", i)),
			//trace.WithLinks(trace.Link{
			//	SpanContext: sc0,
			//	Attributes: []attribute.KeyValue{
			//		attribute.Bool("start", true),
			//	},
			//}),
			//trace.WithSpanKind(trace.SpanKindInternal),
		)
		fmt.Println(span.IsRecording(), span.SpanContext())

		/*
			span.SetAttributes(attribute.BoolSlice("key", []bool{true, false}))

			span.RecordError(
				errors.New("err"),
				trace.WithAttributes(attribute.Bool("fake", true)),
				trace.WithStackTrace(true),
			)
			span.SetStatus(codes.Error, "errored")
		*/

		func() {
			t := time.NewTicker(time.Second * 3)
			defer t.Stop()

			select {
			case <-ctx.Done():
			case <-t.C:
			}
		}()
		span.End()

		fmt.Println("done")

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
