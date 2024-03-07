package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"unsafe"

	kafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("trace-example")

func producerHandler(kafkaWriter *kafka.Writer) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatalln(err)
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
		err = kafkaWriter.WriteMessages(req.Context(),
			msgs...,
		)

		if err != nil {
			_, err1 := wrt.Write([]byte(err.Error()))
			log.Fatalln(err, err1)
		}
	})
}

func getKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr: kafka.TCP("kafka:9092"),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: 1,
		Async:        true,
		WriteBackoffMax: 1 * time.Millisecond,
	}
}

func getKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"kafka:9092"},
		GroupID:  "some group id",
		Topic:    "topic1",
		ReadBatchTimeout: 1 * time.Millisecond,
	})
}

func reader() {
	reader := getKafkaReader()

	defer reader.Close()
	ctx := context.Background()

	fmt.Println("start consuming ... !!")
	for {
		m, err := reader.ReadMessage(ctx)
		_, span := tracer.Start(ctx, "consumer manual span")
		if err != nil {
			log.Fatalln(err)
		}
		span.SetAttributes(
			attribute.KeyValue{Key: "topic", Value: attribute.StringValue(m.Topic)},
			attribute.KeyValue{Key: "offset", Value: attribute.IntValue(int(m.Offset))},
		)
		fmt.Printf("consumed message at topic:%v partition:%v offset:%v	%s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		span.End()
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

	go reader()

	// Add handle func for producer.
	http.HandleFunc("/", producerHandler(kafkaWriter))

	// Run the web server.
	fmt.Printf("size of message: %d", unsafe.Sizeof(kafka.Message{}))
	fmt.Println("start producer-api ... !!")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
