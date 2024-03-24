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

package opentelemetry

import (
	"context"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

// Controller handles OpenTelemetry telemetry generation for events.
type Controller struct {
	logger         logr.Logger
	version        string
	tracerProvider trace.TracerProvider
	tracersMap     map[string]trace.Tracer
	bootTime       int64
}

func (c *Controller) getTracer(pkg string) trace.Tracer {
	t, exists := c.tracersMap[pkg]
	if exists {
		return t
	}

	newTracer := c.tracerProvider.Tracer(
		"go.opentelemetry.io/auto/"+pkg,
		trace.WithInstrumentationVersion(c.version),
	)
	c.tracersMap[pkg] = newTracer
	return newTracer
}

// Trace creates a trace span for event.
func (c *Controller) Trace(event *probe.Event) {
	for _, se := range event.SpanEvents {
		c.logger.Info("got event", "kind", event.Kind.String(), "pkg", event.Package, "attrs", se.Attributes)
		ctx := context.Background()

		if se.SpanContext == nil {
			c.logger.Info("got event without context - dropping")
			return
		}

		// TODO: handle remote parent
		if se.ParentSpanContext != nil {
			ctx = trace.ContextWithSpanContext(ctx, *se.ParentSpanContext)
		}

		ctx = ContextWithEBPFEvent(ctx, *se)
		_, span := c.getTracer(event.Package).
			Start(ctx, se.SpanName,
				trace.WithAttributes(se.Attributes...),
				trace.WithSpanKind(event.Kind),
				trace.WithTimestamp(c.convertTime(se.StartTime)))
		span.SetStatus(se.Status.Code, se.Status.Description)
		span.End(trace.WithTimestamp(c.convertTime(se.EndTime)))
	}
}

func (c *Controller) convertTime(t int64) time.Time {
	return time.Unix(0, c.bootTime+t)
}

// NewController returns a new initialized [Controller].
func NewController(logger logr.Logger, tracerProvider trace.TracerProvider, ver string) (*Controller, error) {
	logger = logger.WithName("Controller")

	bt, err := estimateBootTimeOffset()
	if err != nil {
		return nil, err
	}

	return &Controller{
		logger:         logger,
		version:        ver,
		tracerProvider: tracerProvider,
		tracersMap:     make(map[string]trace.Tracer),
		bootTime:       bt,
	}, nil
}

func estimateBootTimeOffset() (bootTimeOffset int64, err error) {
	// The datapath is currently using ktime_get_boot_ns for the pcap timestamp,
	// which corresponds to CLOCK_BOOTTIME. To be able to convert the the
	// CLOCK_BOOTTIME to CLOCK_REALTIME (i.e. a unix timestamp).

	// There can be an arbitrary amount of time between the execution of
	// time.Now() and unix.ClockGettime() below, especially under scheduler
	// pressure during program startup. To reduce the error introduced by these
	// delays, we pin the current Go routine to its OS thread and measure the
	// clocks multiple times, taking only the smallest observed difference
	// between the two values (which implies the smallest possible delay
	// between the two snapshots).
	var minDiff int64 = 1<<63 - 1
	estimationRounds := 25
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for round := 0; round < estimationRounds; round++ {
		var bootTimespec unix.Timespec

		// Ideally we would use __vdso_clock_gettime for both clocks here,
		// to have as little overhead as possible.
		// time.Now() will actually use VDSO on Go 1.9+, but calling
		// unix.ClockGettime to obtain CLOCK_BOOTTIME is a regular system call
		// for now.
		unixTime := time.Now()
		err = unix.ClockGettime(unix.CLOCK_BOOTTIME, &bootTimespec)
		if err != nil {
			return 0, err
		}

		offset := unixTime.UnixNano() - bootTimespec.Nano()
		diff := offset
		if diff < 0 {
			diff = -diff
		}

		if diff < minDiff {
			minDiff = diff
			bootTimeOffset = offset
		}
	}

	return bootTimeOffset, nil
}
