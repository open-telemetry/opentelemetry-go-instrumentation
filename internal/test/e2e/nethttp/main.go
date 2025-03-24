// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package nethttp is a testing application for the [net/http] package.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func hello(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func main() {
	http.HandleFunc("/hello/{id}", hello)
	go func() {
		_ = http.ListenAndServe(":8080", nil) // nolint: gosec  // Testing server.
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://user@localhost:8080/hello/42?query=true#fragment")
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Body: %s\n", string(body))
	_ = resp.Body.Close()

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
