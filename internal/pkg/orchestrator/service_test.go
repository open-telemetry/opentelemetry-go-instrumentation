// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
