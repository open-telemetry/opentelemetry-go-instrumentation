// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

func TestWithServiceName(t *testing.T) {
	const name = "test_serviceName"

	c, err := newConfig(context.Background(), []Option{WithServiceName((name))})
	require.NoError(t, err)

	assert.Contains(t, c.resAttrs, semconv.ServiceName(defaultServiceName()))
	assert.Contains(t, c.resAttrs, semconv.ServiceName(name))

	res := c.resource().Attributes()
	assert.Contains(t, res, semconv.ServiceName(name))
	assert.NotContains(t, res, semconv.ServiceName(defaultServiceName()))
}

func TestWithEnv(t *testing.T) {
	t.Run("OTEL_SERVICE_NAME", func(t *testing.T) {
		const name = "test_service"
		t.Setenv(envServiceNameKey, name)
		c, err := newConfig(context.Background(), []Option{WithEnv()})
		require.NoError(t, err)
		assert.Contains(t, c.resAttrs, semconv.ServiceName(name))
	})

	t.Run("OTEL_RESOURCE_ATTRIBUTES", func(t *testing.T) {
		const name = "test_service"
		t.Setenv(
			envResourceAttrKey,
			fmt.Sprintf("a=b,fubar,%s=%s,foo=bar", semconv.ServiceNameKey, name),
		)
		c, err := newConfig(context.Background(), []Option{WithEnv()})
		require.NoError(t, err)
		assert.Contains(t, c.resAttrs, attribute.String("a", "b"))
		assert.NotContains(t, c.resAttrs, attribute.String("fubar", ""))
		assert.Contains(t, c.resAttrs, semconv.ServiceName(name))
		assert.Contains(t, c.resAttrs, attribute.String("foo", "bar"))
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
		ctx, opts := context.Background(), []Option{WithEnv()}
		_, err := newConfig(ctx, opts)
		require.NoError(t, err)

		assert.Equal(t, slog.LevelDebug, got)

		t.Setenv(envLogLevelKey, "invalid")
		_, err = newConfig(ctx, opts)
		require.ErrorContains(t, err, `parse log level "invalid"`)
	})
}

func TestWithResourceAttributes(t *testing.T) {
	attr0 := semconv.ServiceName(defaultServiceName())
	attr1 := semconv.ServiceName("test_service")
	attr2 := semconv.K8SPodName("test_pod_name")
	attr3 := semconv.K8SNamespaceName("test_namespace_name")

	want := []attribute.KeyValue{attr0, attr1, attr2, attr3}

	t.Run("Code", func(t *testing.T) {
		opts := []Option{
			WithResourceAttributes(attr1, attr2),
			WithResourceAttributes(attr3),
		}
		c, err := newConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, want, c.resAttrs)
	})

	t.Run("Env", func(t *testing.T) {
		t.Setenv(
			"OTEL_RESOURCE_ATTRIBUTES",
			fmt.Sprintf(
				"%s=%s,%s=%s,%s=%s",
				attr1.Key, attr1.Value.AsString(),
				attr2.Key, attr2.Value.AsString(),
				attr3.Key, attr3.Value.AsString(),
			),
		)

		c, err := newConfig(context.Background(), []Option{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, want, c.resAttrs)
	})

	t.Run("CodeAndEnv", func(t *testing.T) {
		t.Setenv(
			"OTEL_RESOURCE_ATTRIBUTES",
			fmt.Sprintf(
				"%s=%s,%s=%s",
				attr1.Key, attr1.Value.AsString(),
				attr2.Key, attr2.Value.AsString(),
			),
		)

		// Use WithResourceAttributes to config the additional resource attributes
		opts := []Option{WithEnv(), WithResourceAttributes(attr3)}
		c, err := newConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, want, c.resAttrs)
	})
}

func TestWithLogger(t *testing.T) {
	l := slog.New(slog.Default().Handler())
	opts := []Option{WithLogger(l)}
	c, err := newConfig(context.Background(), opts)
	require.NoError(t, err)

	assert.Same(t, l, c.logger)
}
