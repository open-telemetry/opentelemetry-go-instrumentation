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
	"encoding/binary"
	"math"
	"os"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

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
	// pkg is the package being instrumented.
	pkg = "go.opentelemetry.io/otel/internal/global"
)

// New returns a new [probe.Probe].
func New(logger logr.Logger) probe.Probe {
	Id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}
	return &probe.Base[bpfObjects, event]{
		Id:     Id,
		Logger: logger.WithName(Id.String()),
		Consts: []probe.Const{
			probe.RegistersABIConst{},
			probe.AllocationConst{},
			probe.KeyValConst{
				Key: "attr_type_invalid",
				Val: uint64(attribute.INVALID),
			},
			probe.KeyValConst{
				Key: "attr_type_bool",
				Val: uint64(attribute.BOOL),
			},
			probe.KeyValConst{
				Key: "attr_type_int64",
				Val: uint64(attribute.INT64),
			},
			probe.KeyValConst{
				Key: "attr_type_float64",
				Val: uint64(attribute.FLOAT64),
			},
			probe.KeyValConst{
				Key: "attr_type_string",
				Val: uint64(attribute.STRING),
			},
			probe.KeyValConst{
				Key: "attr_type_boolslice",
				Val: uint64(attribute.BOOLSLICE),
			},
			probe.KeyValConst{
				Key: "attr_type_int64slice",
				Val: uint64(attribute.INT64SLICE),
			},
			probe.KeyValConst{
				Key: "attr_type_float64slice",
				Val: uint64(attribute.FLOAT64SLICE),
			},
			probe.KeyValConst{
				Key: "attr_type_stringslice",
				Val: uint64(attribute.STRINGSLICE),
			},
			probe.StructFieldConst{
				Key: "tracer_delegate_pos",
				Val: structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracer", "delegate"),
			},
		},
		Uprobes: []probe.Uprobe[bpfObjects]{
			{
				Sym: "go.opentelemetry.io/otel/internal/global.(*tracer).Start",
				Fn:  uprobeTracerStart,
			},
			{
				Sym: "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).End",
				Fn:  uprobeSpanEnd,
			},
			{
				Sym:      "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetAttributes",
				Fn:       uprobeSetAttributes,
				Optional: true,
			},
			{
				Sym:      "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetName",
				Fn:       uprobeSpanSetName,
				Optional: true,
			},
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

func uprobeSpanSetName(name string, exec *link.Executable, target *process.TargetDetails, obj *bpfObjects) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(name)
	if err != nil {
		return nil, err
	}

	opts := &link.UprobeOptions{Address: offset}
	l, err := exec.Uprobe("", obj.UprobeSetName, opts)
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

type attributeKeyVal struct {
	ValLength uint16
	Vtype     uint8
	Reserved  uint8
	Key       [32]byte
	Value     [128]byte
}

type attributesBuffer struct {
	AttrsKv    [16]attributeKeyVal
	ValidAttrs uint8
}

// event represents a manual span created by the user.
type event struct {
	context.BaseSpanProperties
	SpanName   [64]byte
	Attributes attributesBuffer
}

func convertEvent(e *event) *probe.SpanEvent {
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

	return &probe.SpanEvent{
		SpanName:          spanName,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		Attributes:        convertAttributes(e.Attributes),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
	}
}

func convertAttributes(ab attributesBuffer) []attribute.KeyValue {
	var res []attribute.KeyValue
	for i := 0; i < int(ab.ValidAttrs); i++ {
		akv := ab.AttrsKv[i]
		key := unix.ByteSliceToString(akv.Key[:])
		switch akv.Vtype {
		case uint8(attribute.BOOL):
			res = append(res, attribute.Bool(key, akv.Value[0] != 0))
		case uint8(attribute.INT64):
			res = append(res, attribute.Int64(key, int64(binary.LittleEndian.Uint64(akv.Value[:8]))))
		case uint8(attribute.FLOAT64):
			res = append(res, attribute.Float64(key, math.Float64frombits(binary.LittleEndian.Uint64(akv.Value[:8]))))
		case uint8(attribute.STRING):
			res = append(res, attribute.String(key, unix.ByteSliceToString(akv.Value[:])))
		}
	}
	return res
}
