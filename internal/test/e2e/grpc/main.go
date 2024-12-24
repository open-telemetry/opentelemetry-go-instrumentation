// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Adapted from the gRPC helloworld example:
// https://github.com/grpc/grpc-go/tree/70f1a4045da95b93f73b6dbdd7049f3f053c0680/examples/helloworld

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"
)

const port = 1701

type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	tracer := otel.Tracer("go.opentelemetry.io/auto/internal/test/e2e/grpc")
	_, span := tracer.Start(ctx, "SayHello")
	defer span.End()

	span.SetAttributes(attribute.String("name", in.GetName()))
	log.Printf("Received: %v", in.GetName())
	if in.GetName() == "unimplemented" {
		return nil, status.Error(codes.Unimplemented, "unimplmented")
	}
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func main() {
	// Server.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())

	done := make(chan struct{}, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
		done <- struct{}{}
	}()

	// Give time for auto-instrumentation to initialize.
	time.Sleep(5 * time.Second)

	// Client.
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "world"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.GetMessage())

	// Contact the server expecting a server error
	_, err = c.SayHello(ctx, &pb.HelloRequest{Name: "unimplemented"})
	if err == nil {
		log.Fatalf("expected an error but none was received")
	}
	log.Printf("received expected error: %+v", err)

	s.GracefulStop()
	<-done

	// try making a request after the server has stopped to generate an error status
	_, err = c.SayHello(ctx, &pb.HelloRequest{Name: "world"})
	if err == nil {
		log.Fatalf("expected an error but none was returned")
	}
	log.Printf("received expected error: %+v", err)

	// Give time for auto-instrumentation to do the dew.
	time.Sleep(5 * time.Second)
}
