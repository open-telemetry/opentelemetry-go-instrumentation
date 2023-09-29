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
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func TestWithServiceName(t *testing.T) {
	testServiceName := "test_serviceName"

	// Use WithServiceName to config the service name
	c := newInstConfig([]InstrumentationOption{WithServiceName((testServiceName))})
	assert.Equal(t, testServiceName, c.serviceName)

	// No service name provided - check for default value
	c = newInstConfig([]InstrumentationOption{})
	assert.Equal(t, serviceNameDefault, c.serviceName)

	// OTEL_RESOURCE_ATTRIBUTES
	resServiceName := "resValue"
	err := os.Setenv(envResourceAttrKey, fmt.Sprintf("key1=val1,%s=%s", string(semconv.ServiceNameKey), resServiceName))
	if err != nil {
		t.Error(err)
	}
	c = newInstConfig([]InstrumentationOption{WithServiceName((testServiceName))})
	assert.Equal(t, resServiceName, c.serviceName)

	// Add env var to take precedence
	envServiceName := "env_serviceName"
	err = os.Setenv(envServiceNameKey, envServiceName)
	if err != nil {
		t.Error(err)
	}
	c = newInstConfig([]InstrumentationOption{WithServiceName((testServiceName))})
	assert.Equal(t, envServiceName, c.serviceName)
}

func TestWithPID(t *testing.T) {
	// Get a non existing PID
	maxPIDBytes, err := os.ReadFile("/proc/sys/kernel/pid_max")
	if err != nil {
		t.Error(err)
	}

	maxPID, err := strconv.Atoi(strings.TrimSuffix(string(maxPIDBytes), "\n"))
	if err != nil {
		t.Error(err)
	}

	// Non existing PID
	nonExistingPID := maxPID + 1
	c := newInstConfig([]InstrumentationOption{WithPID(nonExistingPID)})
	assert.Nil(t, c.target)

	// Current PID
	currPID := os.Getpid()
	c = newInstConfig([]InstrumentationOption{WithPID(currPID)})
	exe, err := os.Executable()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, exe, c.target.ExePath)

	// Non existing PID but valid target should override it
	c = newInstConfig([]InstrumentationOption{WithPID(nonExistingPID), WithTarget(exe)})
	assert.Equal(t, exe, c.target.ExePath)

	// Non valid target exe but valid PID - PID should override
	c = newInstConfig([]InstrumentationOption{WithTarget("Non_Existing_Process"), WithPID(currPID)})
	assert.Equal(t, exe, c.target.ExePath)
}
