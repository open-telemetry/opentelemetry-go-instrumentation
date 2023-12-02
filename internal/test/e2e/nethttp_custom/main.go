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
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	rw     http.ResponseWriter
	status int
	data   []byte
}

func (r *statusRecorder) Header() http.Header {
	return r.rw.Header()
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	r.data = data
	return len(data), nil
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
}

func logStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{rw: w}

		next.ServeHTTP(rec, r)

		rec.rw.WriteHeader(rec.status)
		_, err := rec.rw.Write(rec.data)
		if err != nil {
			log.Printf("write failed %s\n", err.Error())
			return
		}

		log.Printf("response status: %d\n", rec.status)
	})
}

func hello(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "hello\n")
}

var tr = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

type MyRoundTripper struct{}

func (rt *MyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-My-Header", "my-value")

	// send the request using the custom transport
	res, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// process the response as needed
	return res, nil
}

func main() {
	go func() {
		_ = http.ListenAndServe(":8080", logStatus(http.HandlerFunc(hello)))
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	req, err := http.NewRequest("GET", "http://localhost:8080/hello", nil)
	if err != nil {
		log.Fatal(err)
		return
	}

	mt := &MyRoundTripper{}

	resp, err := mt.RoundTrip(req)
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
