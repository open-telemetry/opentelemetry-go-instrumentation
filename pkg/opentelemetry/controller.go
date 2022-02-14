package opentelemetry

import (
	"context"
	"fmt"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors/events"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"os"
	"time"
)

const (
	otelEndpointEnvVar    = "OTEL_EXPORTER_OTLP_ENDPOINT"
	otelServiceNameEnvVar = "OTEL_SERVICE_NAME"
)

type Controller struct {
	tracerProvider trace.TracerProvider
	tracersMap     map[string]trace.Tracer
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

func (c *Controller) Trace(event *events.Event) {
	log.Logger.V(0).Info("got event", "attrs", event.Attributes)
	ctx := context.Background()
	_, span := c.getTracer(event.Library).
		Start(ctx, event.Name,
			trace.WithAttributes(event.Attributes...),
			trace.WithSpanKind(event.Kind))
	span.End()
}

func NewController() (*Controller, error) {
	endpoint, exists := os.LookupEnv(otelEndpointEnvVar)
	if !exists {
		return nil, fmt.Errorf("%s env var must be set", otelEndpointEnvVar)
	}

	serviceName, exists := os.LookupEnv(otelServiceNameEnvVar)
	if !exists {
		return nil, fmt.Errorf("%s env var must be set", otelServiceNameEnvVar)
	}

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.TelemetrySDKLanguageGo,
		),
	)
	if err != nil {
		return nil, err
	}

	log.Logger.V(0).Info("Establishing connection to OpenTelemetry collector ...")
	timeoutContext, _ := context.WithTimeout(ctx, time.Second*10)
	conn, err := grpc.DialContext(timeoutContext, endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Logger.Error(err, "unable to connect to OpenTelemetry collector", "addr", endpoint)
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)

	if err != nil {
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	return &Controller{
		tracerProvider: tracerProvider,
		tracersMap:     make(map[string]trace.Tracer),
	}, nil
}
