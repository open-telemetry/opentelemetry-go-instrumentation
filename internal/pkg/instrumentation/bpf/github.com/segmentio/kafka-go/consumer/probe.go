// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"fmt"
	"log/slog"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "github.com/segmentio/kafka-go"
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindConsumer,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "message_headers_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Headers"),
			},
			probe.StructFieldConst{
				Key: "message_key_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Key"),
			},
			probe.StructFieldConst{
				Key: "message_topic_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Topic"),
			},
			probe.StructFieldConst{
				Key: "message_partition_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Partition"),
			},
			probe.StructFieldConst{
				Key: "message_offset_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Offset"),
			},
			probe.StructFieldConst{
				Key: "reader_config_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Reader", "config"),
			},
			probe.StructFieldConst{
				Key: "reader_config_group_id_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "ReaderConfig", "GroupID"),
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:         "github.com/segmentio/kafka-go.(*Reader).FetchMessage",
				EntryProbe:  "uprobe_FetchMessage",
				ReturnProbe: "uprobe_FetchMessage_Returns",
			},
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

// event represents a kafka message received by the consumer.
type event struct {
	context.BaseSpanProperties
	Topic         [256]byte
	Key           [256]byte
	ConsumerGroup [128]byte
	Offset        int64
	Partition     int64
}

func convertEvent(e *event) []*probe.SpanEvent {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	var pscPtr *trace.SpanContext
	if e.ParentSpanContext.TraceID.IsValid() {
		psc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    e.ParentSpanContext.TraceID,
			SpanID:     e.ParentSpanContext.SpanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		pscPtr = &psc
	} else {
		pscPtr = nil
	}

	topic := unix.ByteSliceToString(e.Topic[:])

	attributes := []attribute.KeyValue{
		semconv.MessagingSystemKafka,
		semconv.MessagingOperationTypeReceive,
		semconv.MessagingDestinationPartitionID(strconv.Itoa(int(e.Partition))),
		semconv.MessagingDestinationName(topic),
		semconv.MessagingKafkaMessageOffset(int(e.Offset)),
		semconv.MessagingKafkaMessageKey(unix.ByteSliceToString(e.Key[:])),
		semconv.MessagingKafkaConsumerGroup(unix.ByteSliceToString(e.ConsumerGroup[:])),
	}
	return []*probe.SpanEvent{
		{
			SpanName:          kafkaConsumerSpanName(topic),
			StartTime:         utils.BootOffsetToTime(e.StartTime),
			EndTime:           utils.BootOffsetToTime(e.EndTime),
			SpanContext:       &sc,
			ParentSpanContext: pscPtr,
			Attributes:        attributes,
			TracerSchema:      semconv.SchemaURL,
		},
	}
}

func kafkaConsumerSpanName(topic string) string {
	return fmt.Sprintf("%s receive", topic)
}
