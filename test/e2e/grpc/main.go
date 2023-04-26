package main

import (
	"context"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

type server struct {
	helloworld.UnimplementedGreeterServer
}

func (s server) SayHello(context.Context, *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return &helloworld.HelloReply{
		Message: "hello",
	}, nil
}

func main() {
	address := "0.0.0.0:8090"
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error %e", err)
	}
	s := grpc.NewServer()
	helloworld.RegisterGreeterServer(s, &server{})
	go s.Serve(lis)

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	opts := grpc.WithInsecure()
	conn, err := grpc.Dial("localhost:8090", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := helloworld.NewGreeterClient(conn)
	request := &helloworld.HelloRequest{}

	resp, _ := client.SayHello(context.Background(), request)
	log.Printf("Body: %s", resp.Message)

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
