// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package nethttp_custom is a testing application for the [net/http] package.
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
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint: gosec  // Testing server.
}

// MyRoundTripper implements RoundTripper.
type MyRoundTripper struct{}

// RoundTrip implements RoundTripper.RoundTrip.
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
		_ = http.ListenAndServe(":8080", logStatus(http.HandlerFunc(hello))) // nolint: gosec  // Testing server.
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/hello", nil)
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
