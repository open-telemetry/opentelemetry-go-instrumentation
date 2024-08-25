// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"
)

func writeSpanContextToStdout(ctx context.Context, name string) {
	sc := trace.SpanContextFromContext(ctx)

	spanContextJSON := map[string]string{
		"spanID":     sc.SpanID().String(),
		"traceID":    sc.TraceID().String(),
		"traceFlags": strconv.FormatUint(uint64(sc.TraceFlags()), 16),
	}

	b, _ := json.Marshal(spanContextJSON)

	fmt.Printf("SpanContext of %s: %s\n", name, b)
}

func hello(w http.ResponseWriter, r *http.Request) {
	writeSpanContextToStdout(r.Context(), "server")
	fmt.Fprintf(w, "hello\n")
}

func main() {
	http.HandleFunc("/hello/{id}", hello)
	go func() {
		_ = http.ListenAndServe(":8080", nil)
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
