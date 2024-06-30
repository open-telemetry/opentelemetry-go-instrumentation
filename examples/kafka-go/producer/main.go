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
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	kafka "github.com/segmentio/kafka-go"
)
type server struct {
	kafkaWriter *kafka.Writer
}

func (s *server) producerHandler(wrt http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("failed to read request body: %v\n", err)
		wrt.WriteHeader(http.StatusBadRequest)
		return
	}
	msg1 := kafka.Message{
		Key:   []byte("key1"),
		Value: body,
		Topic: "topic1",
		Headers: []kafka.Header{
			{
				Key:   "header1",
				Value: []byte("value1"),
			},
		},
	}
	msg2 := kafka.Message{
		Key:   []byte("key2"),
		Topic: "topic2",
		Value: body,
	}
	msgs := []kafka.Message{msg1, msg2}
	err = s.kafkaWriter.WriteMessages(req.Context(),
		msgs...,
	)

	if err != nil {
		_, err1 := wrt.Write([]byte(err.Error()))
		if err1 != nil {
			fmt.Printf("failed to write response: %v\n", err1)
			wrt.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	fmt.Fprintf(wrt, "message sent to kafka")
}

func getKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:            kafka.TCP("kafka:9092"),
		Balancer:        &kafka.LeastBytes{},
		RequiredAcks:    1,
		Async:           true,
		WriteBackoffMax: 1 * time.Millisecond,
		BatchTimeout:    1 * time.Millisecond,
	}
}

func main() {
	kafkaWriter := getKafkaWriter()
	defer kafkaWriter.Close()

	_, err := kafka.DialLeader(context.Background(), "tcp", "kafka:9092", "topic1", 0)
	if err != nil {
		panic(err.Error())
	}

	_, err = kafka.DialLeader(context.Background(), "tcp", "kafka:9092", "topic2", 0)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(5 * time.Second)

	s := &server{kafkaWriter: kafkaWriter}

	// Add handle func for producer.
	http.HandleFunc("/produce", s.producerHandler)

	// Run the web server.
	fmt.Println("start producer-api ... !!")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
