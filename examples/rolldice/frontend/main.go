// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package frontend is an example application that uses OpenTelemetry APIs
// directly to instrument its functionality.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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

	srv := newServer(ctx, *listenAddr, *userAddr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	log.Printf("Frontend server listening at %s ...", *listenAddr)

	var err error
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
