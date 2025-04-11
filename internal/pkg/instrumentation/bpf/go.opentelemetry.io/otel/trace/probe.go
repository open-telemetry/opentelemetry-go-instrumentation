// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sdk provides an auto-instrumentation probe for the built-in auto-SDK
// in the go.opentelemetry.io/otel/trace package.
package sdk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

// New returns a new [probe.Probe].
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: "go.opentelemetry.io/otel/trace",
	}

	// Minimum version of go.opentelemetry.io/otel/trace that added an
	// auto-instrumentation SDK implementation for non-recording spans.
	otelWithAutoSDK := probe.PackageConstraints{
		Package: "go.opentelemetry.io/otel/trace",
		Constraints: func() *semver.Constraints {
			c, err := semver.NewConstraint(">= 1.35.1-0")
			if err != nil {
				panic(err)
			}
			return c
		}(),
		FailureMode: probe.FailureModeIgnore,
	}

	uprobeTracerProvider := &probe.Uprobe{
		Sym:        "go.opentelemetry.io/otel/trace.noopSpan.tracerProvider",
		EntryProbe: "uprobe_tracerProvider",
		PackageConstraints: []probe.PackageConstraints{
			otelWithAutoSDK,
		},
	}

	c := &converter{
		logger:               logger,
		uprobeTracerProvider: uprobeTracerProvider,
	}
	return &probe.TraceProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.AllocationConst{},
				probe.StructFieldConst{
					Key: "span_context_trace_id_pos",
					ID: structfield.NewID(
						"go.opentelemetry.io/otel",
						"go.opentelemetry.io/otel/trace",
						"SpanContext",
						"traceID",
					),
				},
				probe.StructFieldConst{
					Key: "span_context_span_id_pos",
					ID: structfield.NewID(
						"go.opentelemetry.io/otel",
						"go.opentelemetry.io/otel/trace",
						"SpanContext",
						"spanID",
					),
				},
				probe.StructFieldConst{
					Key: "span_context_trace_flags_pos",
					ID: structfield.NewID(
						"go.opentelemetry.io/otel",
						"go.opentelemetry.io/otel/trace",
						"SpanContext",
						"traceFlags",
					),
				},
			},
			Uprobes: []*probe.Uprobe{
				uprobeTracerProvider,
				{
					Sym:        "go.opentelemetry.io/otel/trace.(*autoTracer).start",
					EntryProbe: "uprobe_Tracer_start",
					PackageConstraints: []probe.PackageConstraints{
						otelWithAutoSDK,
					},
				},
				{
					Sym:        "go.opentelemetry.io/otel/trace.(*autoSpan).ended",
					EntryProbe: "uprobe_Span_ended",
					PackageConstraints: []probe.PackageConstraints{
						otelWithAutoSDK,
					},
				},
			},
			SpecFn:        loadBpf,
			ProcessRecord: c.decodeEvent,
		},
		ProcessFn: c.processFn,
	}
}

type event struct {
	Size     uint32
	SpanData []byte
}

type recordKind uint64

const (
	recordKindTelemetry recordKind = iota
	recordKindConrol
)

type converter struct {
	logger *slog.Logger

	uprobeTracerProvider *probe.Uprobe
}

func (c *converter) decodeEvent(record perf.Record) (*event, error) {
	c.logger.Debug(
		"decoding event",
		"len",
		len(record.RawSample),
		"CPU",
		record.CPU,
		"remaining",
		record.Remaining,
		"lost",
		record.LostSamples,
	)

	reader := bytes.NewReader(record.RawSample)

	var kind recordKind
	err := binary.Read(reader, binary.LittleEndian, &kind)
	if err != nil {
		c.logger.Error("failed read kind", "error", err)
		return nil, err
	}

	var e *event
	switch kind {
	case recordKindTelemetry:
		e = new(event)

		err = binary.Read(reader, binary.LittleEndian, &e.Size)
		if err != nil {
			c.logger.Error("failed to decode size", "error", err)
			break
		}
		c.logger.Debug("decoded size", "size", e.Size)

		e.SpanData = make([]byte, e.Size)
		_, err = reader.Read(e.SpanData)
		if err != nil {
			c.logger.Error("failed to read span data", "error", err)
			break
		}
		c.logger.Debug("decoded span data", "size", e.Size)
	case recordKindConrol:
		if c.uprobeTracerProvider != nil {
			err = c.uprobeTracerProvider.Close()
			c.uprobeTracerProvider = nil
		}
		c.logger.Debug("unloading noopSpan.tracerProvider uprobe")
	default:
		err = fmt.Errorf("unknown record kind: %d", kind)
	}
	return e, err
}

func (c *converter) processFn(e *event) (pcommon.InstrumentationScope, string, ptrace.SpanSlice) {
	var m ptrace.JSONUnmarshaler
	traces, err := m.UnmarshalTraces(e.SpanData[:e.Size])
	if err != nil {
		c.logger.Error("failed to unmarshal span data", "error", err)
		return pcommon.InstrumentationScope{}, "", ptrace.SpanSlice{}
	}

	rs := traces.ResourceSpans()
	if rs.Len() == 0 {
		c.logger.Error("empty ResourceSpans")
		return pcommon.InstrumentationScope{}, "", ptrace.SpanSlice{}
	}

	ss := rs.At(0).ScopeSpans()
	if ss.Len() == 0 {
		c.logger.Error("empty ScopeSpans")
		return pcommon.InstrumentationScope{}, "", ptrace.SpanSlice{}
	}

	s := ss.At(0)
	return s.Scope(), s.SchemaUrl(), s.Spans()
}
