// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	"go.opentelemetry.io/auto/internal/test/trigger"
)

func hello(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func main() {
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	http.HandleFunc("/hello/{id}", hello)
	go func() {
		_ = http.ListenAndServe(":8080", nil)
	}()

	// Wait for auto-instrumentation.
	err := trig.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}

	url := "http://user@localhost:8080/hello/42?query=true#fragment"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Body: %s\n", string(body))
	_ = resp.Body.Close()
}
