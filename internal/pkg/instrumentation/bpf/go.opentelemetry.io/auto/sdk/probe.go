// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"bytes"
	"encoding/binary"
	"log/slog"

	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

type OtelAutoProbe struct {
	*probe.TargetTraceProducingProbe[bpfObjects, event]
}

func (o *OtelAutoProbe) ApplyConfig(c probe.Config) error {
	return nil
}

// New returns a new [probe.GoLibraryTelemetryProbe].
func New(logger *slog.Logger, handler func(ptrace.ScopeSpans)) probe.GoLibraryTelemetryProbe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: "go.opentelemetry.io/auto",
	}
	c := &converter{logger: logger}

	p := &OtelAutoProbe{
		TargetTraceProducingProbe: probe.NewTargetTraceProducingProbe[bpfObjects, event](),
	}
	p.ProbeID = id
	p.Logger = logger
	p.Consts = consts
	p.Uprobes = uprobes
	p.SpecFn = loadBpf
	p.ProcessRecord = c.decodeEvent
	p.ProcessFn = c.processFn
	p.Handler = handler
	return p
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
	var m ptrace.JSONUnmarshaler
	traces, err := m.UnmarshalTraces(e.SpanData[:e.Size])
	if err != nil {
		c.logger.Error("failed to unmarshal span data", "error", err)
		return ptrace.NewScopeSpans()
	}

	rs := traces.ResourceSpans()
	if rs.Len() == 0 {
		c.logger.Error("empty ResourceSpans")
		return ptrace.NewScopeSpans()
	}

	ss := rs.At(0).ScopeSpans()
	if ss.Len() == 0 {
		c.logger.Error("empty ScopeSpans")
		return ptrace.NewScopeSpans()
	}

	return ss.At(0)
}

var (
	consts = []probe.Const{
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
	}

	uprobes = []probe.Uprobe{
		{
			Sym:        "go.opentelemetry.io/auto/sdk.(*tracer).start",
			EntryProbe: "uprobe_Tracer_start",
		},
		{
			Sym:        "go.opentelemetry.io/auto/sdk.(*span).ended",
			EntryProbe: "uprobe_Span_ended",
		},
	}
)
