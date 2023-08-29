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

package versions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

const shell = "bash"

type goListResponse struct {
	Path     string   `json:"Path"`
	Versions []string `json:"versions"`
}

// List returns all locally known version of module with
// moduleName.
func List(moduleName string) func() ([]string, error) {
	return func() ([]string, error) {
		command := fmt.Sprintf("go list -m -json -versions %s", moduleName)
		cmd := exec.Command(shell, "-c", command)

		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		if err := cmd.Run(); err != nil {
			return nil, err
		}

		resp := goListResponse{}
		if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
			return nil, err
		}

		return resp.Versions, nil
	}
}
