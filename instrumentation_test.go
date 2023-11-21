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

package auto

import (
	"context"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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
	assert.Equal(t, c.defualtServiceName(), c.serviceName)
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

func TestWithAdditionalResourceAttributes(t *testing.T) {
	testAttributes := []attribute.KeyValue{
		semconv.K8SContainerName("test_container_name"),
		semconv.K8SPodName("test_pod_name"),
	}

	// Use WithAdditionalResourceAttributes to config the additional resource attributes
	c, err := newInstConfig(context.Background(), []InstrumentationOption{WithAdditionalResourceAttributes(testAttributes)})
	require.NoError(t, err)
	assert.Equal(t, testAttributes, c.additionalResAttrs)
}

func mockEnv(t *testing.T, env map[string]string) {
	orig := lookupEnv
	t.Cleanup(func() { lookupEnv = orig })

	lookupEnv = func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}
