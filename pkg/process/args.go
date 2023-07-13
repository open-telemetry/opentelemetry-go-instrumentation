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

package process

import (
	"fmt"
	"os"
)

// ExePathEnvVar is the environment variable key whose value points to the
// instrumented executable.
const (
	ExePathEnvVar         = "OTEL_GO_AUTO_TARGET_EXE"
	otelServiceNameEnvVar = "OTEL_SERVICE_NAME"
)

// TargetArgs are the binary target information.
type TargetArgs struct {
	ExecPath    string
	ServiceName string
	MonitorAll  bool
}

// Validate validates t and returns an error if not valid.
func (t *TargetArgs) Validate() error {
	if t.MonitorAll {
		return nil
	}
	if t.ExecPath == "" {
		return fmt.Errorf("execPath is nil")
	}
	if t.ServiceName == "" {
		return fmt.Errorf("serviceName is nil")
	}

	return nil
}

// ParseTargetArgs returns TargetArgs for the target pointed to by the
// environment variable OTEL_GO_AUTO_TARGET_EXE.
func ParseTargetArgs() *TargetArgs {
	result := &TargetArgs{}
	// We are reading only one variable for backwards compatibility.
	val, exists := os.LookupEnv(ExePathEnvVar)

	if exists {
		serviceName, _ := os.LookupEnv(otelServiceNameEnvVar)
		result.ExecPath = val
		result.ServiceName = serviceName
	} else {
		result.MonitorAll = true
	}

	return result
}
