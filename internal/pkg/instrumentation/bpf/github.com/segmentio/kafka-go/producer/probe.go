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

package producer

import (
	"fmt"
	"os"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "github.com/segmentio/kafka-go"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindProducer,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger.WithName(id.String()),
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "writer_topic_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Writer", "Topic"),
			},
			probe.StructFieldConst{
				Key: "message_headers_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Headers"),
			},
			probe.StructFieldConst{
				Key: "message_key_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Key"),
			},
			probe.StructFieldConst{
				Key: "message_time_pos",
				Val: structfield.NewID("github.com/segmentio/kafka-go", "github.com/segmentio/kafka-go", "Message", "Time"),
			},
		},
		Uprobes: []probe.Uprobe[bpfObjects]{
			{
				Sym: "github.com/segmentio/kafka-go.(*Writer).WriteMessages",
				Fn:  uprobeWriteMessages,
			},
		},
		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize()*100)
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

func uprobeWriteMessages(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeWriteMessages, opts)
	if err != nil {
		return nil, err
	}

	links := []link.Link{l}

	retOffsets, err := target.GetFunctionReturns(name)
	if err != nil {
		return nil, err
	}

	for _, ret := range retOffsets {
		opts := &link.UprobeOptions{Address: ret}
		l, err := exec.Uprobe("", obj.UprobeWriteMessagesReturns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

type messageAttributes struct {
	SpaID trace.SpanID
	Topic [256]byte
	Key   [256]byte
}

// event represents a batch of kafka messages being sent.
type event struct {
	StartTime         uint64
	EndTime           uint64
	ParentSpanContext context.EBPFSpanContext
	// Same trace id for all the batch
	TraceID trace.TraceID
	// Message specific attributes
	Messages [10]messageAttributes
	// Global topic for the batch
	GlobalTopic [256]byte
	// Number of valid messages in the batch
	ValidMessages uint64
}

func convertEvent(e *event) []*probe.SpanEvent {
	tsc := trace.SpanContextConfig{
		TraceID:    e.TraceID,
		TraceFlags: trace.FlagsSampled,
	}

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

	globalTopic := unix.ByteSliceToString(e.GlobalTopic[:])

	var commonAttrs []attribute.KeyValue = []attribute.KeyValue{semconv.MessagingSystemKafka, semconv.MessagingOperationPublish}
	if len(globalTopic) > 0 {
		commonAttrs = append(commonAttrs, semconv.MessagingDestinationName(globalTopic))
	}

	if e.ValidMessages > 0 {
		commonAttrs = append(commonAttrs, semconv.MessagingBatchMessageCount(int(e.ValidMessages)))
	}

	var res []*probe.SpanEvent
	var msgTopic string
	for i := uint64(0); i < e.ValidMessages; i++ {
		tsc.SpanID = e.Messages[i].SpaID
		sc := trace.NewSpanContext(tsc)
		key := unix.ByteSliceToString(e.Messages[i].Key[:])

		msgAttrs := []attribute.KeyValue{}
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
		msgAttrs = append(msgAttrs, commonAttrs...)

		res = append(res, &probe.SpanEvent{
			SpanName:          kafkaProducerSpanName(msgTopic),
			StartTime:         int64(e.StartTime),
			EndTime:           int64(e.EndTime),
			SpanContext:       &sc,
			Attributes:        msgAttrs,
			ParentSpanContext: pscPtr,
		})
	}

	return res
}

func kafkaProducerSpanName(topic string) string {
	return fmt.Sprintf("%s publish", topic)
}
