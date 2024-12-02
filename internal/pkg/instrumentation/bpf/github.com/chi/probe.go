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

package chi

import (
	"fmt"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
	"log/slog"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "github.com/go-chi/chi/v5"
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger, version string) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindServer,
		InstrumentedPkg: pkg,
	}
	return &probe.SpanProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.RegistersABIConst{},
				probe.StructFieldConst{
					Key: "pctx_ptr_pos",
					Val: structfield.NewID("github.com/go-chi/chi/v5", "github.com/go-chi/chi/v5", "Context", "parentCtx"),
				},
				probe.StructFieldConst{
					Key: "rp_str_pos",
					Val: structfield.NewID("github.com/go-chi/chi/v5", "github.com/go-chi/chi/v5", "Context", "routePattern"),
				},
				probe.StructFieldConst{
					Key: "method_str_pos",
					Val: structfield.NewID("github.com/go-chi/chi/v5", "github.com/go-chi/chi/v5", "Context", "RouteMethod"),
				},
			},
			Uprobes: []probe.Uprobe{
				{
					Sym:         "github.com/go-chi/chi/v5.(*node).FindRoute",
					EntryProbe:  "uprobe_chi_node_FindRoute",
					ReturnProbe: "uprobe_chi_node_FindRoute_Returns",
				},
			},
			SpecFn: loadBpf,
		},
		Version:   version,
		SchemaURL: semconv.SchemaURL,
		ProcessFn: processFn,
	}
}

// event represents an event in the chi server during an HTTP
// request-response.
type event struct {
	context.BaseSpanProperties
	Method      [8]byte
	Path        [128]byte
	PathPattern [128]byte
}

func processFn(e *event) ptrace.SpanSlice {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])
	patternPath := unix.ByteSliceToString(e.PathPattern[:])

	attributes := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLPath(path),
	}

	if patternPath != "" {
		attributes = append(attributes, semconv.HTTPRouteKey.String(patternPath))
	}

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()
	span.SetName(fmt.Sprintf("%s %s", method, patternPath))
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	utils.Attributes(span.Attributes(), attributes...)

	return spans
}
