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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithServiceName(t *testing.T) {
	testServiceName := "test_serviceName"

	// Use WithServiceName to config the service name
	c := newInstConfig([]InstrumentationOption{WithServiceName((testServiceName))})
	assert.Equal(t, testServiceName, c.serviceName)

	// No service name provided - check for default value
	c = newInstConfig([]InstrumentationOption{})
	assert.Equal(t, serviceNameDefault, c.serviceName)

	// Add env var to take precedence
	envServiceName := "env_serviceName"
	err := os.Setenv(envServiceNameKey, envServiceName)
	if err != nil {
		t.Error(err)
	}
	c = newInstConfig([]InstrumentationOption{WithServiceName((testServiceName))})
	assert.Equal(t, envServiceName, c.serviceName)
}
