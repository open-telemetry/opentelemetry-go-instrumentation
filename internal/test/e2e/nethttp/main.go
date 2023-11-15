// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	rw     http.ResponseWriter
	status int
}

func (r *statusRecorder) Header() http.Header {
	return r.rw.Header()
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.rw.Write(data)
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.rw.WriteHeader(code)
}

func logStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{rw: w}

		next.ServeHTTP(rec, r)

		log.Printf("response status: %d\n", rec.status)
	})
}

func hello(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func main() {
	go func() {
		_ = http.ListenAndServe(":8080", logStatus(http.HandlerFunc(hello)))
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/hello")
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
