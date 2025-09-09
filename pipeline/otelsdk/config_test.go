// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/detectors/autodetect"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
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

type detector struct {
	res *resource.Resource
	err error
}

func (d *detector) Detect(ctx context.Context) (*resource.Resource, error) {
	if d.res == nil {
		return resource.Empty(), d.err
	}
	return d.res, d.err
}

func TestWithResourceDetector(t *testing.T) {
	want := resource.Empty()
	d := &detector{res: want}

	opts := []Option{WithResourceDetector(d)}
	c, err := newConfig(context.Background(), opts)
	require.NoError(t, err)

	// Check that the detector resource was created (not empty).
	require.Len(t, c.detectorResources, 1)
	assert.Equal(t, want, c.detectorResources[0])
}

func TestWithResourceDetectorError(t *testing.T) {
	wantErr := errors.New("test detector error")
	errorDetector := &detector{err: wantErr}

	opts := []Option{WithResourceDetector(errorDetector)}
	_, err := newConfig(context.Background(), opts)
	assert.ErrorIs(t, err, wantErr)
}

func TestWithEnvResourceDetectors(t *testing.T) {
	const id = "TestWithEnvResourceDetectors"
	t.Setenv(envResourceDetectorsKey, id)

	autodetect.Register(id, func() resource.Detector {
		return &detector{}
	})

	c, err := newConfig(context.Background(), []Option{WithEnv()})
	require.NoError(t, err)

	// Check that the detector resource was created (not empty)
	assert.Len(t, c.detectorResources, 1)
	assert.NotNil(t, c.detectorResources[0])
}

func TestWithEnvResourceDetectorsError(t *testing.T) {
	const id = "invalid_detector_that_does_not_exist"
	t.Setenv(envResourceDetectorsKey, id)

	_, err := newConfig(context.Background(), []Option{WithEnv()})
	assert.ErrorContains(t, err, "create autodetect detector")
}

func TestMultipleResourceDetectors(t *testing.T) {
	const id = "TestWithEnvResourceDetectors0"
	res0 := resource.NewSchemaless(attribute.Int("key", 0))
	d0 := &detector{res: res0}
	autodetect.Register(id, func() resource.Detector { return d0 })
	t.Setenv(envResourceDetectorsKey, id)

	res1 := resource.NewSchemaless(attribute.Int("key", 1))
	d1 := &detector{res: res1}

	c, err := newConfig(context.Background(), []Option{
		WithEnv(),
		WithResourceDetector(d1),
	})
	require.NoError(t, err)
	assert.Len(t, c.detectorResources, 2)

	got := c.resource()
	require.NotNil(t, got)

	var want *resource.Resource
	for _, r := range []*resource.Resource{c.baseResource(), res0, res1} {
		var err error
		want, err = resource.Merge(want, r)
		require.NoError(t, err, "merging resources")
	}
	assert.Equal(t, want, got, "merged resource should match expected")
}
