// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"go.opentelemetry.io/auto/internal/test/trigger"
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
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	setup := flag.String("setup", "./start.sh", "Kafka setup script")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Wait for auto-instrumentation.
	err := trig.Wait(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd := exec.Command(*setup) // nolint: gosec  // Testing script.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("starting Kafka...")
	if err := cmd.Run(); err != nil {
		fmt.Printf("failed to start Kafka: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("started Kafka")

	kafkaWriter := getKafkaWriter()
	defer kafkaWriter.Close()

	// to create topics when auto.create.topics.enable='true'
	fmt.Println("trying to connect to kafka")
	for range time.Tick(5 * time.Second) {
		_, err := kafka.DialLeader(ctx, "tcp", "127.0.0.1:9092", "topic1", 0)
		if err == nil {
			break
		}
		fmt.Println("failed to connect to kafka, retrying...")
	}

	fmt.Println("successfully connected to kafka")
	_, err = kafka.DialLeader(ctx, "tcp", "127.0.0.1:9092", "topic2", 0)
	if err != nil {
		panic(err.Error())
	}

	readChan := make(chan bool)
	go reader(readChan)

	produceMessages(kafkaWriter)
	<-readChan
}
