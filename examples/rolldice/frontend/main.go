// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

var (
	listenAddr = flag.String("addr", ":8080", "server listen address")
	userAddr   = flag.String("user", "http://localhost:8082", "user service address")
)

func main() {
	flag.Parse()

	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	shutdown, err := installSDK(ctx)
	if err != nil {
		log.Fatal("Failed to setup OTel:", err)
	}
	defer shutdown()

	srv := newServer(ctx, *listenAddr, *userAddr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	log.Printf("Frontend server listening at %s ...", *listenAddr)

	select {
	case err = <-errCh:
	case <-ctx.Done():
		err = srv.Shutdown(context.Background())
	}
	if err != nil {
		log.Print("Frontend server error:", err)
	}
	log.Print("Frontend server stopped.")
}

func installSDK(ctx context.Context) (func(), error) {
	// Propagator.
	p := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(p)

	// TracerProvider.
	exp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return func() {}, err
	}
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Print("Faild to shutdown TracerProvider:", err)
		}
	}, nil
}
