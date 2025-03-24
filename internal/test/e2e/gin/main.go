// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package gin is a testing application for the [github.com/gin-gonic/gin]
// package.
package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/hello-gin", func(c *gin.Context) {
		c.String(http.StatusOK, "hello\n")
	})
	go func() {
		_ = r.Run()
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/hello-gin")
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
