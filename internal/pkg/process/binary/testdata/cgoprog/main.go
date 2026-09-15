// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Program cgoprog is a minimal HTTP server that uses cgo. It is used to build
// externally linked test binaries, where the C runtime precedes the Go text
// and runtime.text is not the start of the .text section.
package main

/*
static int answer(void) { return 42; }
*/
import "C"

import (
	"net/http"
	"os"
	"strconv"
)

func handler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(strconv.Itoa(int(C.answer()))))
}

func main() {
	http.HandleFunc("/", handler)
	if len(os.Args) > 1 {
		_ = http.ListenAndServe(os.Args[1], nil)
	}
}
