// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Program prog is a minimal HTTP server used to build test binaries for the
// binary package tests.
package main

import (
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func main() {
	http.HandleFunc("/", handler)
	if len(os.Args) > 1 {
		_ = http.ListenAndServe(os.Args[1], nil)
	}
}
