// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package consumer provides an instrumentation probe for Kafka consumer using
// the [github.com/segmentio/kafka-go] package.
package consumer

import (
	"log/slog"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/kernel"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/pdataconv"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "github.com/segmentio/kafka-go"
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindConsumer,
		InstrumentedPkg: pkg,
	}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.AllocationConst{},
				probe.StructFieldConst{
					Key: "message_headers_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Message",
						"Headers",
					),
				},
				probe.StructFieldConst{
					Key: "message_key_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Message",
						"Key",
					),
				},
				probe.StructFieldConst{
					Key: "message_topic_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Message",
						"Topic",
					),
				},
				probe.StructFieldConst{
					Key: "message_partition_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Message",
						"Partition",
					),
				},
				probe.StructFieldConst{
					Key: "message_offset_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Message",
						"Offset",
					),
				},
				probe.StructFieldConst{
					Key: "reader_config_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"Reader",
						"config",
					),
				},
				probe.StructFieldConst{
					Key: "reader_config_group_id_pos",
					ID: structfield.NewID(
						"github.com/segmentio/kafka-go",
						"github.com/segmentio/kafka-go",
						"ReaderConfig",
						"GroupID",
					),
				},
			},
			Uprobes: []*probe.Uprobe{
				{
					Sym:         "github.com/segmentio/kafka-go.(*Reader).FetchMessage",
					EntryProbe:  "uprobe_FetchMessage",
					ReturnProbe: "uprobe_FetchMessage_Returns",
				},
			},
			SpecFn: loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
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

func processFn(e *event) ptrace.SpanSlice {
	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()

	topic := unix.ByteSliceToString(e.Topic[:])
	span.SetName(kafkaConsumerSpanName(topic))
	span.SetKind(ptrace.SpanKindConsumer)

	span.SetStartTimestamp(kernel.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(kernel.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	pdataconv.Attributes(
		span.Attributes(),
		semconv.MessagingSystemKafka,
		semconv.MessagingOperationTypeReceive,
		semconv.MessagingDestinationPartitionID(strconv.Itoa(int(e.Partition))),
		semconv.MessagingDestinationName(topic),
		semconv.MessagingKafkaOffsetKey.Int64(e.Offset),
		semconv.MessagingKafkaMessageKey(unix.ByteSliceToString(e.Key[:])),
		semconv.MessagingConsumerGroupName(unix.ByteSliceToString(e.ConsumerGroup[:])),
	)

	return spans
}

func kafkaConsumerSpanName(topic string) string {
	return topic + " receive"
}
