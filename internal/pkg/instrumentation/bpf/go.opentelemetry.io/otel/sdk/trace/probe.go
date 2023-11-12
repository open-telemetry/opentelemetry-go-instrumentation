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

package sdk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "go.opentelemetry.io/otel/sdk/trace"


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

// Event represents a manual span created by the user
type Event struct {
	context.BaseSpanProperties
	SpanName [64]byte
	Attributes attributesBuffer
}

// Probe is the go.opentelemetry.io/otel/sdk/trace instrumentation probe.
type Probe struct {
	logger       logr.Logger
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Probe].
func New(logger logr.Logger) *Probe {
	return &Probe{logger: logger.WithName("Probe/otel")}
}

// LibraryName returns the /otel/sdk/trace package name.
func (h *Probe) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "go.opentelemetry.io/otel/sdk/trace" that are instrumented.
func (h *Probe) FuncNames() []string {
	return []string{
		"go.opentelemetry.io/otel/sdk/trace.(*tracer).Start",
		"go.opentelemetry.io/otel/sdk/trace.(*recordingSpan).End",
	}
}

// Load loads all instrumentation offsets.
func (h *Probe) Load(exec *link.Executable, target *process.TargetDetails) error {
	const otelSdkMod = "go.opentelemetry.io/otel/sdk"
	otelSdkVer := target.Libraries[otelSdkMod]

	spec, err := loadBpf()
	if err != nil {
		return err
	}
	if target.AllocationDetails == nil {
		// This Probe requires allocation.
		return errors.New("no allocation details")
	}
	err = inject.Constants(
		spec,
		inject.WithRegistersABI(target.IsRegistersABI()),
		inject.WithOffset(
			"span_name_pos",
			structfield.NewID(otelSdkMod, "go.opentelemetry.io/otel/sdk/trace", "recordingSpan", "name"),
			otelSdkVer,
		),
		inject.WithOffset(
			"span_attributes_pos",
			structfield.NewID(otelSdkMod, "go.opentelemetry.io/otel/sdk/trace", "recordingSpan", "attributes"),
			otelSdkVer,
		),
	)
	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}

	err = utils.LoadEBPFObjects(spec, h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(target),
		},
	})

	if err != nil {
		return err
	}

	retOffsets, err := target.GetFunctionReturns(h.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := exec.Uprobe("", h.bpfObjects.UprobeStartReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}

	offset, err := target.GetFunctionOffset(h.FuncNames()[1])
	if err != nil {
		return err
	}

	up, err := exec.Uprobe("", h.bpfObjects.UprobeEnd, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		return err
	}

	h.uprobes = append(h.uprobes, up)

	// Getting the attribute defined from the user might require more memory
	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize() * 8)
	if err != nil {
		return err
	}
	h.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (h *Probe) Run(eventsChan chan<- *probe.Event) {
	var event Event
	for {
		record, err := h.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			h.logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			h.logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			h.logger.Error(err, "error parsing perf event")
			continue
		}

		eventsChan <- h.convertEvent(&event)
	}
}

func (h *Probe) convertEvent(e *Event) *probe.Event {
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
		Library:     h.LibraryName(),
		Name:        spanName,
		Kind:        trace.SpanKindClient,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		Attributes:  h.convertAttributes(&e.Attributes),
		SpanContext: &sc,
		ParentSpanContext: pscPtr,
	}
}

func (h *Probe) convertAttributes(a *attributesBuffer) []attribute.KeyValue {
	var attributes []attribute.KeyValue
	var keyOffset int = 0
	var numericValuesIndex int = 0
	var strValuesOffset int = 0
	for i := 0; i < 128; i++ {
		if a.Headers[i].Vtype == uint8(attribute.INVALID) {
			break;
		}
		key := unix.ByteSliceToString(a.Keys[keyOffset : ])
		keyOffset += (len(key) + 1)
		switch a.Headers[i].Vtype {
		case uint8(attribute.BOOL):
			attributes = append(attributes, attribute.Bool(key, a.NumericValues[numericValuesIndex] != 0))
			numericValuesIndex++
			break
		case uint8(attribute.INT64):
			attributes = append(attributes, attribute.Int64(key, a.NumericValues[numericValuesIndex]))
			numericValuesIndex++
			break
		case uint8(attribute.FLOAT64):
			attributes = append(attributes, attribute.Float64(key, math.Float64frombits(uint64(a.NumericValues[numericValuesIndex]))))
			numericValuesIndex++
			break
		case uint8(attribute.STRING):
			strVal := unix.ByteSliceToString(a.StrValues[strValuesOffset : ])
			attributes = append(attributes, attribute.String(key, strVal))
			strValuesOffset += (len(strVal) + 1)
			break
		// TODO: handle slices
		default:
			break
		}
	}
	return attributes
}


// Close stops the Probe.
func (h *Probe) Close() {
	h.logger.Info("closing go.opentelemetry.io/otel/sdk/trace probe")
	if h.eventsReader != nil {
		h.eventsReader.Close()
	}

	for _, r := range h.uprobes {
		r.Close()
	}

	for _, r := range h.returnProbs {
		r.Close()
	}

	if h.bpfObjects != nil {
		h.bpfObjects.Close()
	}
}
