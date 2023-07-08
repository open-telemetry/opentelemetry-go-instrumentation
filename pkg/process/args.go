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

type ExeService struct {
	ExecPath    string
	ServiceName string
}

// TargetArgs are the binary target information.
type TargetArgs struct {
	IgnoreProcesses  map[string]any
	IncludeProcesses map[string]ExeService
}

// Validate validates t and returns an error if not valid.
func (t *TargetArgs) Validate() error {
	if t.MonitorAll() {
		return nil
	}
	for k, v := range t.IncludeProcesses {
		if v.ExecPath == "" || v.ServiceName == "" {
			return fmt.Errorf("execPath or serviceName is nil for %v", k)
		}
	}

	return nil
}

func (t *TargetArgs) MonitorAll() bool {
	return len(t.IncludeProcesses) == 0
}

// ParseTargetArgs returns TargetArgs for the target pointed to by the
// environment variable OTEL_GO_AUTO_TARGET_EXE.
func ParseTargetArgs() *TargetArgs {
	ignoreProcesses := make(map[string]any)
	ignoreProcesses["docker"] = nil
	ignoreProcesses["dockerd"] = nil
	ignoreProcesses["containerd"] = nil
	ignoreProcesses["gopls"] = nil
	ignoreProcesses["docker-proxy"] = nil
	ignoreProcesses["otel-go-instrumentation"] = nil
	ignoreProcesses["gops"] = nil
	ignoreProcesses["containerd-shim-runc-v2"] = nil
	ignoreProcesses["coredns"] = nil
	ignoreProcesses["kindnetd"] = nil
	ignoreProcesses["kubelet"] = nil
	ignoreProcesses["kube-scheduler"] = nil
	ignoreProcesses["otelcol-contrib"] = nil

	result := &TargetArgs{
		IgnoreProcesses:  ignoreProcesses,
		IncludeProcesses: make(map[string]ExeService),
	}
	// We are reading only one variable for backwards compatibility.
	val, exists := os.LookupEnv(ExePathEnvVar)

	if exists {
		serviceName, _ := os.LookupEnv(otelServiceNameEnvVar)
		result.IncludeProcesses[val] = ExeService{
			ExecPath:    val,
			ServiceName: serviceName,
		}
	}

	return result
}
