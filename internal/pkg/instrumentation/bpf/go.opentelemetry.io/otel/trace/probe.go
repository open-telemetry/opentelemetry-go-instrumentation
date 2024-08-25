// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"github.com/go-logr/logr"

	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -no-global-types -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "go.opentelemetry.io/otel/trace"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, struct{}]{
		ID:     id,
		Logger: logger.WithName(id.String()),
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.StructFieldConst{
				Key: "span_context_traceID_offset",
				Val: structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/trace", "SpanContext", "traceID"),
			},
			probe.StructFieldConst{
				Key: "span_context_spanID_offset",
				Val: structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/trace", "SpanContext", "spanID"),
			},
			probe.StructFieldConst{
				Key: "span_context_traceFlags_offset",
				Val: structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/trace", "SpanContext", "traceFlags"),
			},
		},
		Uprobes: []probe.Uprobe{
			{
				Sym:         "go.opentelemetry.io/otel/trace.SpanContextFromContext",
				EntryProbe:  "uprobe_SpanContextFromContext",
				ReturnProbe: "uprobe_SpanContextFromContext_Returns",
			},
		},
		SpecFn:    loadBpf,
		ProcessFn: nil,
	}
}
