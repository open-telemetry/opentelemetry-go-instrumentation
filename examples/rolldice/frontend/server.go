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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/examples/rolldice/user"
)

const scope = "go.opentelemetry.io/auto/examples/rolldice/frontend"

type serviceKeyType int

const userKey serviceKeyType = 0

func newServer(ctx context.Context, listenAddr, userAddr string) *http.Server {
	sNameOpt := otelhttp.WithSpanNameFormatter(formatter)
	t := otelhttp.NewTransport(http.DefaultTransport, sNameOpt)

	client := user.NewClient(&http.Client{Transport: t}, userAddr)
	if err := client.HealthCheck(ctx); err != nil {
		log.Print("Cannot reach User service: ", err)
	} else {
		log.Print("Connected to User service at ", userAddr)
	}
	ctx = context.WithValue(ctx, userKey, client)

	mux := http.NewServeMux()

	handle(mux, "/rolldice/{player}", http.HandlerFunc(rolldice))

	return &http.Server{
		Addr:        listenAddr,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
		Handler:     otelhttp.NewHandler(mux, "/rolldice/{player}", sNameOpt),
	}
}

func formatter(op string, r *http.Request) string {
	name := r.Method
	if name == "" {
		name = http.MethodGet
	}

	if op != "" {
		name += " " + op
	} else if r.Pattern != "" {
		name += " " + r.Pattern
	}

	return name
}

func handle(mux *http.ServeMux, pattern string, handler http.Handler) {
	mux.Handle(pattern, otelhttp.WithRouteTag(pattern, handler))
}

func rolldice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(scope)
	_, span := tracer.Start(r.Context(), "rolldice")
	defer span.End()

	player := r.PathValue("player")

	client, ok := ctx.Value(userKey).(*user.Client)
	if !ok {
		http.Error(w, "Internal Error", http.StatusInternalServerError)

		err := errors.New("no User client")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	if err := client.UseQuota(ctx, player); err != nil {
		if errors.Is(err, user.ErrInsufficient) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, "Internal Error", http.StatusInternalServerError)
		}
		return
	}

	roll := 1 + rand.Intn(6)

	if player != "" {
		span.SetAttributes(attribute.String("player", player))
	}
	span.SetAttributes(attribute.Int("roll.value", roll))

	resp := strconv.Itoa(roll) + "\n"
	if _, err := io.WriteString(w, resp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
