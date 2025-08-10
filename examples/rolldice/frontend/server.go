// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/auto/examples/rolldice/user"
)

type serviceKeyType int

const userKey serviceKeyType = 0

func newServer(ctx context.Context, listenAddr, userAddr string) *http.Server {
	client := user.NewClient(http.DefaultClient, userAddr)
	if err := client.HealthCheck(ctx); err != nil {
		log.Print("Cannot reach User service: ", err)
	} else {
		log.Print("Connected to User service at ", userAddr)
	}
	ctx = context.WithValue(ctx, userKey, client)

	mux := http.NewServeMux()
	mux.HandleFunc("/rolldice/{player}", rolldice)

	return &http.Server{
		Addr:              listenAddr,
		ReadHeaderTimeout: time.Second,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
		Handler:           mux,
	}
}

func rolldice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	player := r.PathValue("player")

	client, ok := ctx.Value(userKey).(*user.Client)
	if !ok {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	if err := client.UseQuota(ctx, player); err != nil {
		if errors.Is(err, user.ErrInsufficient) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		} else {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
		}
		return
	}

	roll := 1 + rand.Intn(6) //nolint:gosec  // Weak random number generator is fine.
	_, _ = io.WriteString(w, strconv.Itoa(roll)+"\n")
}
