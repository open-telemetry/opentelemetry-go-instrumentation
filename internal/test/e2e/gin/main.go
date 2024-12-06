// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/auto/internal/test/trigger"
)

func main() {
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r := gin.Default()
	r.GET("/hello-gin", func(c *gin.Context) {
		c.String(http.StatusOK, "hello\n")
	})
	go func() {
		_ = r.Run()
	}()

	// Wait for auto-instrumentation.
	err := trig.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}

	url := "http://localhost:8080/hello-gin"
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
