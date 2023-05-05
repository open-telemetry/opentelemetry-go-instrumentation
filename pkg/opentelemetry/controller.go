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
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	otelServiceNameEnvVar = "OTEL_SERVICE_NAME"
)

var (
	// Controller-local reference to the auto-instrumentation release version.
	releaseVersion = auto.Version()
	// Start of this auto-instrumentation's exporter User-Agent header, e.g. ""OTel-Go-Auto-Instrumentation/1.2.3".
	baseUserAgent = fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", releaseVersion)
	// Information about the runtime environment for inclusion in User-Agent, e.g. "go/1.18.2 (linux/amd64)".
	runtimeInfo = fmt.Sprintf("%s (%s/%s)", strings.Replace(runtime.Version(), "go", "go/", 1), runtime.GOOS, runtime.GOARCH)
	// Combined User-Agent identifying this auto-instrumentation and its runtime environment, see RFC7231 for format considerations.
	autoinstUserAgent = fmt.Sprintf("%s %s", baseUserAgent, runtimeInfo)
)

// Controller handles OpenTelemetry telemetry generation for events.
type Controller struct {
	tracerProvider trace.TracerProvider
	tracersMap     map[string]trace.Tracer
	bootTime       int64
}

func (c *Controller) getTracer(libName string) trace.Tracer {
	t, exists := c.tracersMap[libName]
	if exists {
		return t
	}

	newTracer := c.tracerProvider.Tracer(libName)
	c.tracersMap[libName] = newTracer
	return newTracer
}

// Trace creates a trace span for event.
func (c *Controller) Trace(event *events.Event) {
	log.Logger.V(0).Info("got event", "attrs", event.Attributes)
	ctx := context.Background()

	if event.SpanContext == nil {
		log.Logger.V(0).Info("got event without context - dropping")
		return
	}

	// TODO: handle remote parent
	if event.ParentSpanContext != nil {
		ctx = trace.ContextWithSpanContext(ctx, *event.ParentSpanContext)
	}

	ctx = ContextWithEBPFEvent(ctx, *event)
	_, span := c.getTracer(event.Library).
		Start(ctx, event.Name,
			trace.WithAttributes(event.Attributes...),
			trace.WithSpanKind(event.Kind),
			trace.WithTimestamp(c.convertTime(event.StartTime)))
	span.End(trace.WithTimestamp(c.convertTime(event.EndTime)))
}

func (c *Controller) convertTime(t int64) time.Time {
	return time.Unix(0, c.bootTime+t)
}

// NewController returns a new initialized [Controller].
func NewController() (*Controller, error) {
	serviceName, exists := os.LookupEnv(otelServiceNameEnvVar)
	if !exists {
		return nil, fmt.Errorf("%s env var must be set", otelServiceNameEnvVar)
	}

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.TelemetrySDKLanguageGo,
			semconv.TelemetryAutoVersionKey.String(releaseVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	log.Logger.V(0).Info("Establishing connection to OTLP receiver ...")
	otlpTraceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithDialOption(grpc.WithUserAgent(autoinstUserAgent)),
	)
	traceExporter, err := otlptrace.New(ctx, otlpTraceClient)
	if err != nil {
		log.Logger.Error(err, "unable to connect to OTLP endpoint")
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithIDGenerator(newEBPFSourceIDGenerator()),
	)

	bt, err := estimateBootTimeOffset()
	if err != nil {
		return nil, err
	}

	return &Controller{
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
