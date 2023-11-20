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

package global

import (
	"math"
	"os"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	// name is the instrumentation name.
	name = "go.opentelemetry.io/otel/internal/global"
	// pkg is the package being instrumented.
	pkg = "go.opentelemetry.io/otel/internal/global"
	// maxAttributes is the maximum number of attributes that can be added to a span.
	maxAttributes = 128
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	return &probe.Base[bpfObjects, event]{
		Name:            name,
		Logger:          logger.WithName(name),
		InstrumentedPkg: pkg,
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
		},
		Uprobes: map[string]probe.UprobeFunc[bpfObjects]{
			"go.opentelemetry.io/otel/internal/global.(*tracer).Start":                   uprobeTracerStart,
			"go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetAttributes": uprobeSetAttributes,
			"go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).End":           uprobeSpanEnd,
		},

		ReaderFn: func(obj bpfObjects) (*perf.Reader, error) {
			return perf.NewReader(obj.Events, os.Getpagesize()*8)
		},
		SpecFn:    loadBpf,
		ProcessFn: convertEvent,
	}
}

func uprobeTracerStart(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeStart, opts)
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
		l, err := exec.Uprobe("", obj.UprobeStartReturns, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	return links, nil
}

func uprobeSetAttributes(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeSetAttributes, opts)
	if err != nil {
		return nil, err
	}

	links := []link.Link{l}

	return links, nil
}

func uprobeSpanEnd(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeEnd, opts)
	if err != nil {
		return nil, err
	}

	links := []link.Link{l}

	return links, nil
}

type attributeHeader struct {
	ValLength uint16
	Vtype     uint8
	Reserved  uint8
}

type attributesBuffer struct {
	Headers       [128]attributeHeader
	Keys          [256]byte
	NumericValues [32]int64
	StrValues     [1024]byte
}

// event represents a manual span created by the user.
type event struct {
	context.BaseSpanProperties
	SpanName   [64]byte
	Attributes attributesBuffer
}

func convertEvent(e *event) *probe.Event {
	spanName := unix.ByteSliceToString(e.SpanName[:])

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

	return &probe.Event{
		Package:           pkg,
		Name:              spanName,
		Kind:              trace.SpanKindClient,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		Attributes:        convertAttributes(&e.Attributes),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
	}
}

func convertAttributes(a *attributesBuffer) []attribute.KeyValue {
	var attributes []attribute.KeyValue
	var keyOffset int
	var numericValuesIndex int
	var strValuesOffset int
	for i := 0; i < maxAttributes; i++ {
		if a.Headers[i].Vtype == uint8(attribute.INVALID) {
			break
		}
		key := unix.ByteSliceToString(a.Keys[keyOffset:])
		keyOffset += (len(key) + 1)
		switch a.Headers[i].Vtype {
		case uint8(attribute.BOOL):
			attributes = append(attributes, attribute.Bool(key, a.NumericValues[numericValuesIndex] != 0))
			numericValuesIndex++
		case uint8(attribute.INT64):
			attributes = append(attributes, attribute.Int64(key, a.NumericValues[numericValuesIndex]))
			numericValuesIndex++
		case uint8(attribute.FLOAT64):
			attributes = append(attributes, attribute.Float64(key, math.Float64frombits(uint64(a.NumericValues[numericValuesIndex]))))
			numericValuesIndex++
		case uint8(attribute.STRING):
			strVal := unix.ByteSliceToString(a.StrValues[strValuesOffset:])
			attributes = append(attributes, attribute.String(key, strVal))
			strValuesOffset += (len(strVal) + 1)
		// TODO: handle slices
		default:
		}
	}
	return attributes
}
