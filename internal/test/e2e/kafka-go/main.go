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
	"time"

	kafka "github.com/segmentio/kafka-go"
)

func produceMessages(kafkaWriter *kafka.Writer) {
	msg1 := kafka.Message{
		Key:   []byte("key1"),
		Value: []byte("value1"),
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
		Value: []byte("value2"),
		Topic: "topic2",
	}
	msgs := []kafka.Message{msg1, msg2}
	err := kafkaWriter.WriteMessages(context.Background(),
		msgs...,
	)
	if err != nil {
		fmt.Printf("failed to write messages: %v\n", err)
	}
}

func getKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:            kafka.TCP("127.0.0.1:9092"),
		Balancer:        &kafka.LeastBytes{},
		Async:           true,
		RequiredAcks:    1,
		WriteBackoffMax: 1 * time.Millisecond,
		BatchTimeout:    1 * time.Millisecond,
	}
}

func getKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:          []string{"127.0.0.1:9092"},
		GroupID:          "some group id",
		Topic:            "topic1",
		ReadBatchTimeout: 1 * time.Millisecond,
	})
}

func reader(readChan chan bool) {
	reader := getKafkaReader()

	defer reader.Close()

	fmt.Println("start consuming ... !!")
	for {
		_, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("failed to read message: %v\n", err)
		}
		readChan <- true
	}
}

func main() {
	kafkaWriter := getKafkaWriter()
	defer kafkaWriter.Close()

	// to create topics when auto.create.topics.enable='true'
	fmt.Println("trying to connect to kafka")
	for range time.Tick(5 * time.Second) {
		_, err := kafka.DialLeader(context.Background(), "tcp", "127.0.0.1:9092", "topic1", 0)
		if err == nil {
			break
		}
		fmt.Println("failed to connect to kafka, retrying...")
	}

	fmt.Println("successfully connected to kafka")
	_, err := kafka.DialLeader(context.Background(), "tcp", "127.0.0.1:9092", "topic2", 0)
	if err != nil {
		panic(err.Error())
	}

	readChan := make(chan bool)
	go reader(readChan)

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	produceMessages(kafkaWriter)
	<-readChan

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
