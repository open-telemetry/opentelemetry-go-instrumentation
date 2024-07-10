// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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
	ctx := context.Background()

	c, err := newInstConfig(ctx, []InstrumentationOption{WithPID(1)})
	require.NoError(t, err)
	assert.Equal(t, 1, c.target.Pid)

	const exe = "./test/path/program/run.go"
	// PID should override valid target exe
	c, err = newInstConfig(ctx, []InstrumentationOption{WithTarget(exe), WithPID(1)})
	require.NoError(t, err)
	assert.Equal(t, 1, c.target.Pid)
	assert.Equal(t, "", c.target.ExePath)
}

func TestWithEnv(t *testing.T) {
	t.Run("OTEL_GO_AUTO_TARGET_EXE", func(t *testing.T) {
		const path = "./test/path/program/run.go"
		mockEnv(t, map[string]string{"OTEL_GO_AUTO_TARGET_EXE": path})
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, path, c.target.ExePath)
		assert.Equal(t, 0, c.target.Pid)
	})

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
		const name = "debug"
		mockEnv(t, map[string]string{"OTEL_LOG_LEVEL": name})

		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		assert.Equal(t, LogLevelDebug, c.logLevel)

		const wrong = "invalid"

		mockEnv(t, map[string]string{"OTEL_LOG_LEVEL": wrong})
		_, err = newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.Error(t, err)
	})
}

func TestOptionPrecedence(t *testing.T) {
	const (
		path = "./test/path/program/run.go"
		name = "test_service"
	)

	t.Run("Env", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_GO_AUTO_TARGET_EXE": path,
			"OTEL_SERVICE_NAME":       name,
		})

		// WithEnv passed last, it should have precedence.
		opts := []InstrumentationOption{
			WithPID(1),
			WithServiceName("wrong"),
			WithEnv(),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, path, c.target.ExePath)
		assert.Equal(t, 0, c.target.Pid)
		assert.Equal(t, name, c.serviceName)
	})

	t.Run("Options", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_GO_AUTO_TARGET_EXE": path,
			"OTEL_SERVICE_NAME":       "wrong",
		})

		// WithEnv passed first, it should be overridden.
		opts := []InstrumentationOption{
			WithEnv(),
			WithPID(1),
			WithServiceName(name),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.Equal(t, "", c.target.ExePath)
		assert.Equal(t, 1, c.target.Pid)
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

func TestWithLogLevel(t *testing.T) {
	t.Run("With Valid Input", func(t *testing.T) {
		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithLogLevel("error")})

		require.NoError(t, err)

		assert.Equal(t, LogLevelError, c.logLevel)

		c, err = newInstConfig(context.Background(), []InstrumentationOption{WithLogLevel(LogLevelInfo)})

		require.NoError(t, err)

		assert.Equal(t, LogLevelInfo, c.logLevel)
	})

	t.Run("Will Validate Input", func(t *testing.T) {
		_, err := newInstConfig(context.Background(), []InstrumentationOption{WithLogLevel("invalid")})

		require.Error(t, err)
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
