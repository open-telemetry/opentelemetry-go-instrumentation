// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"log/slog"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
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
		SpanKind:        trace.SpanKindProducer,
		InstrumentedPkg: pkg,
	}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.RegistersABIConst{},
				probe.AllocationConst{},
				probe.StructFieldConst{
					Key: "writer_topic_pos",
					ID:  structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Writer", "Topic"),
				},
				probe.StructFieldConst{
					Key: "message_headers_pos",
					ID:  structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Headers"),
				},
				probe.StructFieldConst{
					Key: "message_key_pos",
					ID:  structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Key"),
				},
				probe.StructFieldConst{
					Key: "message_time_pos",
					ID:  structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Time"),
				},
			},
			Uprobes: []*probe.Uprobe{
				{
					Sym:         "github.com/segmentio/kafka-go.(*Writer).WriteMessages",
					EntryProbe:  "uprobe_WriteMessages",
					ReturnProbe: "uprobe_WriteMessages_Returns",
				},
			},
			SpecFn: loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

type messageAttributes struct {
	SpanContext context.EBPFSpanContext
	Topic       [256]byte
	Key         [256]byte
}

// event represents a batch of kafka messages being sent.
type event struct {
	StartTime         uint64
	EndTime           uint64
	ParentSpanContext context.EBPFSpanContext
	// Message specific attributes
	Messages [10]messageAttributes
	// Global topic for the batch
	GlobalTopic [256]byte
	// Number of valid messages in the batch
	ValidMessages uint64
}

func processFn(e *event) ptrace.SpanSlice {
	globalTopic := unix.ByteSliceToString(e.GlobalTopic[:])

	attrs := []attribute.KeyValue{semconv.MessagingSystemKafka, semconv.MessagingOperationTypePublish}
	if len(globalTopic) > 0 {
		attrs = append(attrs, semconv.MessagingDestinationName(globalTopic))
	}

	if e.ValidMessages > 0 {
		e.ValidMessages = min(e.ValidMessages, math.MaxInt)
		attrs = append(attrs, semconv.MessagingBatchMessageCount(int(e.ValidMessages))) // nolint: gosec  // Bounded.
	}

	traceID := pcommon.TraceID(e.Messages[0].SpanContext.TraceID)

	spans := ptrace.NewSpanSlice()

	var msgTopic string
	for i := uint64(0); i < e.ValidMessages; i++ {
		key := unix.ByteSliceToString(e.Messages[i].Key[:])
		var msgAttrs []attribute.KeyValue
		if len(key) > 0 {
			msgAttrs = append(msgAttrs, semconv.MessagingKafkaMessageKey(key))
		}

		// Topic is either the global topic or the message specific topic
		if len(globalTopic) == 0 {
			msgTopic = unix.ByteSliceToString(e.Messages[i].Topic[:])
		} else {
			msgTopic = globalTopic
		}

		msgAttrs = append(msgAttrs, semconv.MessagingDestinationName(msgTopic))
		msgAttrs = append(msgAttrs, attrs...)

		span := spans.AppendEmpty()
		span.SetName(kafkaProducerSpanName(msgTopic))
		span.SetKind(ptrace.SpanKindProducer)
		span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
		span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID(e.Messages[i].SpanContext.SpanID))
		span.SetFlags(uint32(trace.FlagsSampled))

		if e.ParentSpanContext.SpanID.IsValid() {
			span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
		}

		utils.Attributes(span.Attributes(), msgAttrs...)
	}

	return spans
}

func kafkaProducerSpanName(topic string) string {
	return topic + " publish"
}
