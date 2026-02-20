// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const service = "handler_test"

var (
	pAttrs = func() pcommon.Map {
		m := pcommon.NewMap()
		m.PutBool("bool", true)
		m.PutInt("int64", 43)
		m.PutDouble("float64", -1.238)
		m.PutStr("string.a", "a")
		m.PutStr("string.b", "b")
		_ = m.PutEmptySlice("bool.slice").FromRaw([]any{true, false, true})
		_ = m.PutEmptySlice("int.slice").FromRaw([]any{-2, 2, 34})
		_ = m.PutEmptySlice("float64.slice").FromRaw([]any{-2., .0832, 43e12})
		_ = m.PutEmptySlice("string.slice").FromRaw([]any{"x", "y", "z"})
		return m
	}()

	oAttrs = func() []attribute.KeyValue {
		out := []attribute.KeyValue{
			attribute.Bool("bool", true),
			attribute.Int64("int64", 43),
			attribute.Float64("float64", -1.238),
			attribute.String("string.a", "a"),
			attribute.String("string.b", "b"),
			attribute.BoolSlice("bool.slice", []bool{true, false, true}),
			attribute.IntSlice("int.slice", []int{-2, 2, 34}),
			attribute.Float64Slice("float64.slice", []float64{-2., .0832, 43e12}),
			attribute.StringSlice("string.slice", []string{"x", "y", "z"}),
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].Key < out[j].Key
		})
		return out
	}()
)

var defaultRes = func() *resource.Resource {
	c, err := newConfig(context.Background(), []Option{
		WithServiceName(service),
	})
	if err != nil {
		panic(err)
	}
	r, err := resource.Merge(resource.Environment(), c.resource())
	if err != nil {
		panic(err)
	}
	return r
}()

func TestTraceHandlerHandleTrace(t *testing.T) {
	const schemaURL = "http://localhost/1.0.0"

	scope := pcommon.NewInstrumentationScope()

	const scopeName = "go.opentelemetry.io/auto/pipeline/otelsdk/test"
	scope.SetName(scopeName)
	const scopeVer = "v0.0.1"
	scope.SetVersion(scopeVer)
	pAttrs.CopyTo(scope.Attributes())

	spans := ptrace.NewSpanSlice()
	span := spans.AppendEmpty()

	const spanName = "test.span"
	span.SetName(spanName)

	tid := trace.TraceID{0x1}
	span.SetTraceID(pcommon.TraceID(tid))
	sid := trace.SpanID{0x1}
	span.SetSpanID(pcommon.SpanID(sid))
	const spanFlags = 1
	span.SetFlags(spanFlags)

	span.SetKind(ptrace.SpanKindClient)

	startTime := time.Unix(0, 0).UTC()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	endTime := time.Unix(1, 0).UTC()
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	pAttrs.CopyTo(span.Attributes())

	ctx := context.Background()
	exp := newExporter()
	handler, err := NewTraceHandler(ctx, WithTraceExporter(exp), WithServiceName(service))
	require.NoError(t, err)

	// Note: DroppedAttributesCount not supported.
	link := span.Links().AppendEmpty()
	link.SetTraceID(pcommon.TraceID(tid))
	link.SetSpanID(pcommon.SpanID(sid))
	link.SetFlags(1)
	pAttrs.CopyTo(link.Attributes())

	event := span.Events().AppendEmpty()
	const eventName = "event"
	event.SetName(eventName)
	pAttrs.CopyTo(event.Attributes())
	event.SetTimestamp(pcommon.NewTimestampFromTime(startTime))

	handler.HandleTrace(scope, schemaURL, spans)
	require.NoError(t, handler.Shutdown(ctx))
	got := exp.GetSpans()

	wantScope := instrumentation.Scope{
		Name:       scopeName,
		Version:    scopeVer,
		SchemaURL:  schemaURL,
		Attributes: attribute.NewSet(oAttrs...),
	}
	want := tracetest.SpanStubs{
		{
			Resource:               defaultRes,
			InstrumentationLibrary: wantScope,
			InstrumentationScope:   wantScope,
			Name:                   spanName,
			SpanKind:               trace.SpanKindClient,
			StartTime:              startTime,
			EndTime:                endTime,
			Attributes:             oAttrs,
			Links: []sdktrace.Link{
				{
					SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
						TraceID:    tid,
						SpanID:     sid,
						TraceFlags: 1,
					}),
					Attributes: oAttrs,
				},
			},
			Events: []sdktrace.Event{
				{
					Name:       eventName,
					Attributes: oAttrs,
					Time:       startTime,
				},
			},
		},
	}
	assert.Len(t, got, len(want))

	for i, span := range got {
		// Span contexts get modified by exporter, update expected with output.
		want[i].SpanContext = span.SpanContext

		sort.Slice(span.Attributes, func(i, j int) bool {
			return span.Attributes[i].Key < span.Attributes[j].Key
		})

		for _, link := range span.Links {
			sort.Slice(link.Attributes, func(i, j int) bool {
				return link.Attributes[i].Key < link.Attributes[j].Key
			})
		}

		for _, e := range span.Events {
			sort.Slice(e.Attributes, func(i, j int) bool {
				return e.Attributes[i].Key < e.Attributes[j].Key
			})
		}
	}
	assert.Equal(t, want, got)
}

type exporter struct {
	*tracetest.InMemoryExporter
}

func newExporter() *exporter {
	return &exporter{
		InMemoryExporter: tracetest.NewInMemoryExporter(),
	}
}

func (exporter) Shutdown(context.Context) error {
	// Override InMemoryExporter behavior.
	return nil
}

type shutdownExporter struct {
	sdktrace.SpanExporter

	exported atomic.Uint32
	called   bool
}

// ExportSpans handles export of spans by storing them in memory.
func (e *shutdownExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	n := len(spans)
	if n < 0 || n > math.MaxUint32 {
		return fmt.Errorf("invalid span length: %d", n)
	}
	e.exported.Add(uint32(n)) //nolint:gosec  // Bound checked
	return nil
}

func (e *shutdownExporter) Shutdown(context.Context) error {
	e.called = true
	return nil
}

func TestTraceHandlerShutdown(t *testing.T) {
	const nSpan = 10

	exp := new(shutdownExporter)

	t.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", strconv.Itoa(nSpan+1))
	t.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", strconv.Itoa(nSpan+1))
	// Ensure we are checking Shutdown flushes the queue.
	t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "36000")

	ctx := context.Background()
	handler, err := NewTraceHandler(ctx, WithTraceExporter(exp))
	require.NoError(t, err)

	for i := range nSpan {
		scope := pcommon.NewInstrumentationScope()
		scope.SetName("test")

		spans := ptrace.NewSpanSlice()
		span := spans.AppendEmpty()
		span.SetName("span" + strconv.Itoa(i))
		span.SetTraceID(pcommon.TraceID{0x1})
		span.SetSpanID(pcommon.SpanID{0x1})
		handler.HandleTrace(scope, "", spans)
	}

	require.NoError(t, handler.Shutdown(ctx))
	assert.True(t, exp.called, "Exporter not shutdown")
	assert.Equal(t, uint32(nSpan), exp.exported.Load(), "Pending spans not flushed")
}

func TestControllerTraceConcurrentSafe(t *testing.T) {
	handler, err := NewTraceHandler(context.Background())
	assert.NoError(t, err)

	const goroutines = 10

	var wg sync.WaitGroup
	for n := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			scope := pcommon.NewInstrumentationScope()
			scope.SetName(fmt.Sprintf("tracer-%d", n%(goroutines/2)))
			scope.SetVersion("v1")
			spans := ptrace.NewSpanSlice()
			span := spans.AppendEmpty()
			span.SetName("test")
			span.SetTraceID(pcommon.TraceID{0x1})
			span.SetSpanID(pcommon.SpanID{0x1})
			handler.HandleTrace(scope, "url", spans)
		}()
	}

	wg.Wait()
}
