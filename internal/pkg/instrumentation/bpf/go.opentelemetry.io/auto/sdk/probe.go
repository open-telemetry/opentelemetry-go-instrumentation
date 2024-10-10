// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"bytes"
	"encoding/binary"
	"log/slog"

	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// New returns a new [probe.Probe].
func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: "go.opentelemetry.io/auto",
	}
	c := &converter{logger: logger}
	return &probe.Base[bpfObjects, event]{
		ID:     id,
		Logger: logger,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.StructFieldConst{
				Key: "span_context_trace_id_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"traceID",
				),
			},
			probe.StructFieldConst{
				Key: "span_context_span_id_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"spanID",
				),
			},
			probe.StructFieldConst{
				Key: "span_context_trace_flags_pos",
				Val: structfield.NewID(
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/trace",
					"SpanContext",
					"traceFlags",
				),
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:        "go.opentelemetry.io/auto/sdk.(*tracer).start",
				EntryProbe: "uprobe_Tracer_start",
			},
			{
				Sym:        "go.opentelemetry.io/auto/sdk.(*span).ended",
				EntryProbe: "uprobe_Span_ended",
			},
		},
		SpecFn:        loadBpf,
		ProcessFn:     c.processFn,
		ProcessRecord: c.decodeEvent,
	}
}

type event struct {
	Size     uint32
	SpanData []byte
}

type converter struct {
	logger *slog.Logger
}

func (c *converter) decodeEvent(record perf.Record) (event, error) {
	reader := bytes.NewReader(record.RawSample)

	var e event
	err := binary.Read(reader, binary.LittleEndian, &e.Size)
	if err != nil {
		c.logger.Error("failed to decode size", "error", err)
		return event{}, err
	}
	c.logger.Debug("decoded size", "size", e.Size)

	e.SpanData = make([]byte, e.Size)
	_, err = reader.Read(e.SpanData)
	if err != nil {
		c.logger.Error("failed to read span data", "error", err)
		return event{}, err
	}
	c.logger.Debug("decoded span data", "size", e.Size)
	return e, nil
}

func (c *converter) processFn(e *event) ptrace.ScopeSpans {
	var m ptrace.ProtoUnmarshaler
	traces, err := m.UnmarshalTraces(e.SpanData[:e.Size])
	if err != nil {
		c.logger.Error("failed to unmarshal span data", "error", err)
		return ptrace.NewScopeSpans()
	}

	return traces.ResourceSpans().At(0).ScopeSpans().At(0) // TODO: validate len before lookup.
}
