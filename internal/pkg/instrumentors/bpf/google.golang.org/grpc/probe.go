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

package grpc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/offsets"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

// Event represents an event in the gRPC client during a gRPC request.
type Event struct {
	context.BaseSpanProperties
	Method [50]byte
	Target [50]byte
}

// Instrumentor is the gRPC client instrumentor.
type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	eventsReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the gRPC package import path.
func (g *Instrumentor) LibraryName() string {
	return "google.golang.org/grpc"
}

// FuncNames returns the function names from "google.golang.org/grpc" that are
// instrumented.
func (g *Instrumentor) FuncNames() []string {
	return []string{
		"google.golang.org/grpc.(*ClientConn).Invoke",
		"google.golang.org/grpc/internal/transport.(*http2Client).NewStream",
		"google.golang.org/grpc/internal/transport.(*loopyWriter).headerHandler",
	}
}

// Load loads all instrumentation offsets.
func (g *Instrumentor) Load(exec *link.Executable, target *process.TargetDetails) error {
	v := target.Libraries[g.LibraryName()]
	ver, err := version.NewVersion(v)
	if err != nil {
		return fmt.Errorf("invalid package version: %w", err)
	}

	spec, err := loadBpf()
	if err != nil {
		return err
	}
	if target.AllocationDetails == nil {
		// This Instrumentor requires allocation.
		return errors.New("no allocation details")
	}
	err = inject.Constants(
		spec,
		inject.WithRegistersABI(target.IsRegistersABI()),
		inject.WithAllocationDetails(*target.AllocationDetails),
		inject.WithOffset(
			"clientconn_target_ptr_pos",
			offsets.NewID("google.golang.org/grpc", "ClientConn", "target"),
			ver,
		),
		inject.WithOffset(
			"httpclient_nextid_pos",
			offsets.NewID("google.golang.org/grpc/internal/transport", "http2Client", "nextID"),
			ver,
		),
		inject.WithOffset(
			"headerFrame_hf_pos",
			offsets.NewID("google.golang.org/grpc/internal/transport", "headerFrame", "hf"),
			ver,
		),
		inject.WithOffset(
			"headerFrame_streamid_pos",
			offsets.NewID("google.golang.org/grpc/internal/transport", "headerFrame", "streamID"),
			ver,
		),
	)
	if err != nil {
		return err
	}

	g.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(target),
		},
	})

	if err != nil {
		return err
	}

	offset, err := target.GetFunctionOffset(g.FuncNames()[0])
	if err != nil {
		return err
	}

	up, err := exec.Uprobe("", g.bpfObjects.UprobeClientConnInvoke, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		return err
	}

	g.uprobes = append(g.uprobes, up)

	retOffsets, err := target.GetFunctionReturns(g.FuncNames()[0])
	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := exec.Uprobe("", g.bpfObjects.UprobeClientConnInvokeReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		g.uprobes = append(g.uprobes, retProbe)
	}

	// SendMsg probe
	sendMsgOffset, err := target.GetFunctionOffset(g.FuncNames()[1])
	if err != nil {
		return err
	}
	sendMsgProbe, err := exec.Uprobe("", g.bpfObjects.UprobeHttp2ClientNewStream, &link.UprobeOptions{
		Address: sendMsgOffset,
	})
	if err != nil {
		return err
	}
	g.uprobes = append(g.uprobes, sendMsgProbe)

	// Write headers probe
	whOffset, err := target.GetFunctionOffset(g.FuncNames()[2])
	if err != nil {
		return err
	}

	whProbe, err := exec.Uprobe("", g.bpfObjects.UprobeLoopyWriterHeaderHandler, &link.UprobeOptions{
		Address: whOffset,
	})
	if err != nil {
		return err
	}

	g.uprobes = append(g.uprobes, whProbe)

	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd
	return nil
}

// Run runs the events processing loop.
func (g *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("grpc-instrumentor")
	var event Event
	for {
		record, err := g.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			logger.Error(err, "error parsing perf event")
			continue
		}

		eventsChan <- g.convertEvent(&event)
	}
}

// According to https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func (g *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	target := unix.ByteSliceToString(e.Target[:])
	var attrs []attribute.KeyValue

	// remove port
	if parts := strings.Split(target, ":"); len(parts) > 1 {
		target = parts[0]
		attrs = append(attrs, semconv.NetPeerPortKey.String(parts[1]))
	}

	attrs = append(attrs, semconv.RPCSystemKey.String("grpc"),
		semconv.RPCServiceKey.String(method),
		semconv.NetPeerNameKey.String(target))

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

	log.Logger.V(0).Info("got spancontext", "trace_id", e.SpanContext.TraceID.String(), "span_id", e.SpanContext.SpanID.String())
	return &events.Event{
		Library:           g.LibraryName(),
		Name:              method,
		Kind:              trace.SpanKindClient,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		Attributes:        attrs,
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
	}
}

// Close stops the Instrumentor.
func (g *Instrumentor) Close() {
	log.Logger.V(0).Info("closing gRPC instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	for _, r := range g.uprobes {
		r.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
