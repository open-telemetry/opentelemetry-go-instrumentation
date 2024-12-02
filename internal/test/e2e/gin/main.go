// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func hello(c *gin.Context) {
	c.String(http.StatusOK, "hello\n")
}

func main() {
	r := gin.Default()
	r.GET("/hello-gin/:id", hello)

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
