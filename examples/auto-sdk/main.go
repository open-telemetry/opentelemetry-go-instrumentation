// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/auto/sdk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	service    = "go.opentelemetry.io/auto/examples/auto-sdk"
	listenAddr = "localhost:8080"
)

func main() {
	otel.SetTracerProvider(sdk.GetTracerProvider())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	db, err := openDB()
	if err != nil {
		log.Fatal("database error: ", err)
	}
	if err = initDB(ctx, db); err != nil {
		log.Print("database initialization error: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Print("database close error: ", err)
		}
	}()

	newServer(ctx, listenAddr)
	log.Printf("Starting backend at %s ...", listenAddr)
	srv := newServer(ctx, listenAddr)
	go func() { _ = srv.ListenAndServe() }()
	log.Println("Backend started.")

	log.Println("Starting client")
	run(ctx, listenAddr)
}

func run(ctx context.Context, addr string) error {
	for i := 0; ; i++ {
		client(ctx, addr, i)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func client(ctx context.Context, addr string, i int) {
	fmt.Printf("making request %d...", i)

	tracer := otel.Tracer(service)

	var span trace.Span
	ctx, span = tracer.Start(
		ctx,
		"client",
		trace.WithAttributes(attribute.Int("i", i)),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	url := fmt.Sprintf("http://%s/user/alice/score/%d", addr, i)
	err := doReq(ctx, url)
	if err != nil {
		fmt.Printf("failed request: %s\n", err.Error())
		return
	}

	s := time.Duration(3 + rand.Intn(2))
	wait(ctx, s*time.Second)

	fmt.Println("done")
}

func doReq(ctx context.Context, url string) (err error) {
	var span trace.Span
	ctx, span = otel.Tracer(service).Start(
		ctx,
		"doReq",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	span.AddEvent(
		"score.set",
		trace.WithAttributes(attribute.String("score", string(data))),
	)

	return nil
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
