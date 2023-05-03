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
