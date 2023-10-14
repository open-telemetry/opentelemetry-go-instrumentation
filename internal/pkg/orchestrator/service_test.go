package orchestrator

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func TestWithServiceName(t *testing.T) {
	defer func() {
		_ = os.Unsetenv(envResourceAttrKey)
		_ = os.Unsetenv(envServiceNameKey)
	}()
	testServiceName := "test_serviceName"

	// Use WithServiceName to config the service name
	c, err := New(context.Background(), WithServiceName(testServiceName))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, testServiceName, c.serviceName)

	// No service name provided - check for default value
	c, err = New(context.Background())
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, serviceNameDefault, c.serviceName)

	// OTEL_RESOURCE_ATTRIBUTES
	resServiceName := "resValue"
	err = os.Setenv(envResourceAttrKey, fmt.Sprintf("key1=val1,%s=%s", string(semconv.ServiceNameKey), resServiceName))
	if err != nil {
		t.Error(err)
	}
	c, err = New(context.Background(), WithServiceName((testServiceName)))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, resServiceName, c.serviceName)

	// Add env var to take precedence
	envServiceName := "env_serviceName"
	err = os.Setenv(envServiceNameKey, envServiceName)
	if err != nil {
		t.Error(err)
	}
	c, err = New(context.Background(), WithServiceName((testServiceName)))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, envServiceName, c.serviceName)
}

func TestWithPID(t *testing.T) {
	// Current PID
	currPID := os.Getpid()
	c, err := New(context.Background(), WithPID(currPID))
	if err != nil {
		t.Error(err)
	}
	currExe, err := os.Executable()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, currPID, c.pid)

	// PID should override valid target exe
	c, err = New(context.Background(), WithPID(currPID), WithTarget(currExe))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, currPID, c.pid)
}
