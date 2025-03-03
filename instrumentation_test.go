// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/go-logr/stdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
)

func TestWithServiceName(t *testing.T) {
	ctx := context.Background()
	testServiceName := "test_serviceName"

	// Use WithServiceName to config the service name
	c, err := newInstConfig(ctx, []InstrumentationOption{WithServiceName((testServiceName))})
	require.NoError(t, err)
	assert.Equal(t, testServiceName, c.serviceName)

	// No service name provided - check for default value
	c, err = newInstConfig(ctx, []InstrumentationOption{})
	require.NoError(t, err)
	assert.Equal(t, c.defaultServiceName(), c.serviceName)
}

func TestWithPID(t *testing.T) {
	c, err := newInstConfig(context.Background(), []InstrumentationOption{WithPID(1)})
	require.NoError(t, err)
	assert.Equal(t, 1, c.targetPID)
}

func TestWithEnv(t *testing.T) {
	t.Run("OTEL_SERVICE_NAME", func(t *testing.T) {
		const name = "test_service"
		mockEnv(t, map[string]string{"OTEL_SERVICE_NAME": name})
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, name, c.serviceName)
	})

	t.Run("OTEL_RESOURCE_ATTRIBUTES", func(t *testing.T) {
		const name = "test_service"
		val := fmt.Sprintf("a=b,fubar,%s=%s,foo=bar", semconv.ServiceNameKey, name)
		mockEnv(t, map[string]string{"OTEL_RESOURCE_ATTRIBUTES": val})
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, name, c.serviceName)
	})

	t.Run("OTEL_LOG_LEVEL", func(t *testing.T) {
		orig := newLogger
		var got slog.Leveler
		newLogger = func(level slog.Leveler) *slog.Logger {
			got = level
			return newLoggerFunc(level)
		}
		t.Cleanup(func() { newLogger = orig })

		t.Setenv(envLogLevelKey, "debug")
		ctx, opts := context.Background(), []InstrumentationOption{WithEnv()}
		_, err := newInstConfig(ctx, opts)
		require.NoError(t, err)

		assert.Equal(t, slog.LevelDebug, got)

		t.Setenv(envLogLevelKey, "invalid")
		_, err = newInstConfig(ctx, opts)
		require.ErrorContains(t, err, `parse log level "invalid"`)
	})

	// Test that autoexport.NewSpanExporter works when OTEL_TRACES_EXPORTER is
	// not set and OTEL_EXPORTER_OTLP_PROTOCOL is set to 'grpc'
	t.Run("With OTEL_TRACES_EXPORTER not set", func(t *testing.T) {
		os.Unsetenv("OTEL_TRACES_EXPORTER")
		t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})

		require.NoError(t, err)
		require.NotNil(t, c.traceExp)
		require.IsType(t, &otlptrace.Exporter{}, c.traceExp)
		exp := c.traceExp.(*otlptrace.Exporter)
		var buf bytes.Buffer
		logger := stdr.New(log.New(&buf, "", log.LstdFlags))
		logger.Info("", "exporter", exp)
		got, err := io.ReadAll(&buf)
		require.NoError(t, err)
		assert.Contains(t, string(got), "otlptracegrpc")
	})
}

func TestOptionPrecedence(t *testing.T) {
	const (
		path = "./test/path/program/run.go"
		name = "test_service"
	)

	t.Run("Env", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_SERVICE_NAME": name,
		})

		// WithEnv passed last, it should have precedence.
		opts := []InstrumentationOption{
			WithServiceName("wrong"),
			WithEnv(),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, name, c.serviceName)
	})

	t.Run("Options", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_SERVICE_NAME": "wrong",
		})

		// WithEnv passed first, it should be overridden.
		opts := []InstrumentationOption{
			WithEnv(),
			WithServiceName(name),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, name, c.serviceName)
	})
}

func TestWithResourceAttributes(t *testing.T) {
	t.Run("By Code", func(t *testing.T) {
		attr1 := semconv.K8SContainerName("test_container_name")
		attr2 := semconv.K8SPodName("test_pod_name")
		attr3 := semconv.K8SNamespaceName("test_namespace_name")

		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithResourceAttributes(attr1, attr2), WithResourceAttributes(attr3)})
		require.NoError(t, err)
		assert.Equal(t, []attribute.KeyValue{attr1, attr2, attr3}, c.additionalResAttrs)
	})

	t.Run("By Env", func(t *testing.T) {
		nameAttr := semconv.ServiceName("test_service")
		attr2 := semconv.K8SPodName("test_pod_name")
		attr3 := semconv.K8SNamespaceName("test_namespace_name")

		mockEnv(t, map[string]string{
			"OTEL_RESOURCE_ATTRIBUTES": fmt.Sprintf("%s=%s,%s=%s,%s=%s", nameAttr.Key, nameAttr.Value.AsString(), attr2.Key, attr2.Value.AsString(), attr3.Key, attr3.Value.AsString()),
		})

		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, nameAttr.Value.AsString(), c.serviceName)
		assert.Equal(t, []attribute.KeyValue{attr2, attr3}, c.additionalResAttrs)
	})

	t.Run("By Code and Env", func(t *testing.T) {
		nameAttr := semconv.ServiceName("test_service")
		attr2 := semconv.K8SPodName("test_pod_name")
		attr3 := semconv.K8SNamespaceName("test_namespace_name")

		mockEnv(t, map[string]string{
			"OTEL_RESOURCE_ATTRIBUTES": fmt.Sprintf("%s=%s,%s=%s", nameAttr.Key, nameAttr.Value.AsString(), attr2.Key, attr2.Value.AsString()),
		})

		// Use WithResourceAttributes to config the additional resource attributes
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv(), WithResourceAttributes(attr3)})
		require.NoError(t, err)
		assert.Equal(t, nameAttr.Value.AsString(), c.serviceName)
		assert.Equal(t, []attribute.KeyValue{attr2, attr3}, c.additionalResAttrs)
	})
}

func TestWithLogger(t *testing.T) {
	l := slog.New(slog.Default().Handler())
	opts := []InstrumentationOption{WithLogger(l)}
	c, err := newInstConfig(context.Background(), opts)
	require.NoError(t, err)

	assert.Same(t, l, c.logger)
}

func TestWithSampler(t *testing.T) {
	t.Run("Default sampler", func(t *testing.T) {
		c, err := newInstConfig(context.Background(), []InstrumentationOption{})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.DefaultConfig().Samplers, sc.Samplers)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		conf, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, conf.SamplerType)
		pbConfig, ok := conf.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.DefaultParentBasedSampler(), pbConfig)
	})

	t.Run("Env config", func(t *testing.T) {
		mockEnv(t, map[string]string{
			tracesSamplerKey:    samplerNameParentBasedTraceIDRatio,
			tracesSamplerArgKey: "0.42",
		})

		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		parentBasedConfig, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, parentBasedConfig.SamplerType)
		pbConfig, ok := parentBasedConfig.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.TraceIDRatioID, pbConfig.Root)
		tidRatio, ok := sc.Samplers[sampling.TraceIDRatioID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerTraceIDRatio, tidRatio.SamplerType)
		config, ok := tidRatio.Config.(sampling.TraceIDRatioConfig)
		assert.True(t, ok)
		expected, _ := sampling.NewTraceIDRatioConfig(0.42)
		assert.Equal(t, expected, config)
	})

	t.Run("Invalid Env config", func(t *testing.T) {
		mockEnv(t, map[string]string{
			tracesSamplerKey:    "invalid",
			tracesSamplerArgKey: "0.42",
		})

		_, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown sampler name")
	})

	t.Run("WithSampler", func(t *testing.T) {
		c, err := newInstConfig(context.Background(), []InstrumentationOption{
			WithSampler(ParentBasedSampler{
				Root: TraceIDRatioSampler{Fraction: 0.42},
			}),
		})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		parentBasedConfig, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, parentBasedConfig.SamplerType)
		pbConfig, ok := parentBasedConfig.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.TraceIDRatioID, pbConfig.Root)
		assert.Equal(t, sampling.AlwaysOnID, pbConfig.RemoteSampled)
		assert.Equal(t, sampling.AlwaysOffID, pbConfig.RemoteNotSampled)
		assert.Equal(t, sampling.AlwaysOnID, pbConfig.LocalSampled)
		assert.Equal(t, sampling.AlwaysOffID, pbConfig.LocalNotSampled)

		tidRatio, ok := sc.Samplers[sampling.TraceIDRatioID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerTraceIDRatio, tidRatio.SamplerType)
		config, ok := tidRatio.Config.(sampling.TraceIDRatioConfig)
		assert.True(t, ok)
		expected, _ := sampling.NewTraceIDRatioConfig(0.42)
		assert.Equal(t, expected, config)
	})
}

func mockEnv(t *testing.T, env map[string]string) {
	orig := lookupEnv
	t.Cleanup(func() { lookupEnv = orig })

	lookupEnv = func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}
