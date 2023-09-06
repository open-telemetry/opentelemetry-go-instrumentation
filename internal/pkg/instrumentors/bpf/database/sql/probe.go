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

package sql

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strconv"

	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "database/sql"

// Event represents an event in an SQL database
// request-response.
type Event struct {
	context.BaseSpanProperties
	Query [100]byte
}

// Instrumentor is the database/sql instrumentor.
type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbs  []link.Link
	eventsReader *perf.Reader
}

// IncludeDBStatementEnvVar is the environment variable to opt-in for sql query inclusion in the trace.
const IncludeDBStatementEnvVar = "OTEL_GO_AUTO_INCLUDE_DB_STATEMENT"

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the database/sql/ package name.
func (h *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "database/sql" that are instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"database/sql.(*DB).queryDC"}
}

// Load loads all instrumentation offsets.
func (h *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), nil, []*inject.FlagField{
		{
			VarName: "should_include_db_statement",
			Value:   shouldIncludeDBStatement(),
		}}, true)

	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}

	err = utils.LoadEBPFObjects(spec, h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(ctx.TargetDetails),
		},
	})

	if err != nil {
		return err
	}

	offset, err := ctx.TargetDetails.GetFunctionOffset(h.FuncNames()[0])

	if err != nil {
		return err
	}

	up, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeQueryDC, &link.UprobeOptions{
		Address: offset,
	})

	if err != nil {
		return err
	}

	h.uprobes = append(h.uprobes, up)

	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(h.FuncNames()[0])

	if err != nil {
		return err
	}

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeQueryDC_Returns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			return err
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}

	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	h.eventsReader = rd

	return nil
}

// Run runs the events processing loop.
func (h *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("database/sql/sql-instrumentor")
	var event Event
	for {
		record, err := h.eventsReader.Read()
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

		eventsChan <- h.convertEvent(&event)
	}
}

func (h *Instrumentor) convertEvent(e *Event) *events.Event {
	query := unix.ByteSliceToString(e.Query[:])

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

	return &events.Event{
		Library:     h.LibraryName(),
		Name:        "DB",
		Kind:        trace.SpanKindClient,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.DBStatementKey.String(query),
		},
		ParentSpanContext: pscPtr,
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	log.Logger.V(0).Info("closing database/sql/sql instrumentor")
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

// shouldIncludeDBStatement returns if the user has configured SQL queries to be included.
func shouldIncludeDBStatement() bool {
	val := os.Getenv(IncludeDBStatementEnvVar)
	if val != "" {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}
