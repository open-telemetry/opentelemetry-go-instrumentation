// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package grpc is a testing application for the [google.golang.org/grpc]
// package.
package main

// Adapted from the gRPC helloworld example:
// https://github.com/grpc/grpc-go/tree/70f1a4045da95b93f73b6dbdd7049f3f053c0680/examples/helloworld

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/auto/internal/test/getlog"
	"go.opentelemetry.io/auto/internal/test/trigger"
)

const port = 1701

type server struct {
	logger *slog.Logger
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	tracer := otel.Tracer("go.opentelemetry.io/auto/internal/test/e2e/grpc")
	_, span := tracer.Start(ctx, "SayHello")
	defer span.End()

	span.SetAttributes(attribute.String("name", in.GetName()))
	slog.Debug("received", "name", in.GetName())
	if in.GetName() == "unimplemented" {
		return nil, status.Error(codes.Unimplemented, "unimplmented")
	}
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func main() {
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	var logLvl getlog.Flag
	flag.Var(&logLvl, "log-level", logLvl.Docs())
	flag.Parse()

	logger := logLvl.Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverStop, err := setup(logger)
	if err != nil {
		logger.Error("failed to setup server", "error", err)
		os.Exit(1)
	}

	// Wait for auto-instrumentation.
	err = trig.Wait(ctx)
	if err != nil {
		logger.Error("failed to wait for trigger", "error", err)
		os.Exit(1)
	}

	err = run(ctx, serverStop, logger)
	if err != nil {
		logger.Error("failed to run client", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, srvStop func(), logger *slog.Logger) error {
	// Client.
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "world"})
	if err != nil {
		return fmt.Errorf("could not greet: %w", err)
	}
	logger.Debug("greeting", "msg", r.GetMessage())

	// Contact the server expecting a server error
	_, err = c.SayHello(ctx, &pb.HelloRequest{Name: "unimplemented"})
	if err == nil {
		logger.Error("expected an error but none was returned", "method", "unimplemented")
	} else {
		logger.Debug("received expected error", "error", err)
	}

	srvStop()

	// try making a request after the server has stopped to generate an error
	// status
	_, err = c.SayHello(ctx, &pb.HelloRequest{Name: "world"})
	if err == nil {
		logger.Error("expected an error but none was returned", "server", "stopped")
	} else {
		logger.Debug("received expected error", "error", err)
	}

	return nil
}

func setup(logger *slog.Logger) (stop func(), err error) {
	// Server.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return func() {}, fmt.Errorf("failed to listen: %w", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{logger: logger})
	logger.Debug("listening", "address", lis.Addr())

	done := make(chan struct{}, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Error("server failed", "error", err)
		}
		done <- struct{}{}
	}()

	return func() {
		s.GracefulStop()
		<-done
	}, nil
}
