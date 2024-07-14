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
	"os"
	"os/signal"
	"syscall"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("trace-example-kafka-go", trace.WithInstrumentationVersion("v1.0.0-test"))

func getKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:          []string{"kafka:9092"},
		GroupID:          "some group id",
		Topic:            "topic1",
		ReadBatchTimeout: 1 * time.Millisecond,
	})
}

func reader(ctx context.Context) {
	reader := getKafkaReader()

	defer reader.Close()

	fmt.Println("start consuming ... !!")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			m, err := reader.ReadMessage(ctx)
			if err != nil {
				fmt.Printf("failed to read message: %v\n", err)
				continue
			}
			_, span := tracer.Start(ctx, "consumer manual span")
			span.SetAttributes(
				attribute.String("topic", m.Topic),
				attribute.Int64("partition", int64(m.Partition)),
				attribute.Int64("offset", int64(m.Offset)),
			)
			fmt.Printf("consumed message at topic:%v partition:%v offset:%v	%s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
			span.End()
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	time.Sleep(5 * time.Second)
	go reader(ctx)

	<-ch
	cancel()
}
