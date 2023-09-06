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
	"errors"
	"os"
)

// ExePathEnvVar is the environment variable key whose value points to the
// instrumented executable.
const ExePathEnvVar = "OTEL_GO_AUTO_TARGET_EXE"

// TargetArgs are the binary target information.
type TargetArgs struct {
	ExePath string
}

// Validate validates t and returns an error if not valid.
func (t *TargetArgs) Validate() error {
	if t.ExePath == "" {
		return errors.New("target binary path not specified, please specify " + ExePathEnvVar + " env variable")
	}

	return nil
}

// ParseTargetArgs returns TargetArgs for the target pointed to by the
// environment variable OTEL_GO_AUTO_TARGET_EXE.
func ParseTargetArgs() *TargetArgs {
	result := &TargetArgs{}

	val, exists := os.LookupEnv(ExePathEnvVar)
	if exists {
		result.ExePath = val
	}

	return result
}
