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

package opentelemetry

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestReleaseVersion(t *testing.T) {
	versionYaml, err := ioutil.ReadFile("../../versions.yaml")
	if err != nil {
		t.Fatalf("Couldn't read versions.yaml file: %e", err)
		return
	}

	var versionInfo map[string]interface{}

	err = yaml.Unmarshal(versionYaml, &versionInfo)
	if err != nil {
		t.Fatalf("Couldn't parse version.yaml: %e", err)
		return
	}

	// incredibad, but it's where the intended version is declared at the moment
	expectedVersion := versionInfo["module-sets"].(map[string]interface{})["alpha"].(map[string]interface{})["version"]
	assert.Equal(t, expectedVersion, releaseVersion, "Controller release version should match versions.yaml so that it can report the version in use.")
}
