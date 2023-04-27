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
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// regex taken from https://github.com/Masterminds/semver/tree/v3.1.1
var versionRegex = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?$`)

func TestVersionSemver(t *testing.T) {
	v := Version()
	assert.NotNil(t, versionRegex.FindStringSubmatch(v), "version is not semver: %s", v)
}

func TestVersionMatchesYaml(t *testing.T) {
	versionYaml, err := ioutil.ReadFile("versions.yaml")
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
	assert.Equal(t, expectedVersion, Version(), "Build version should match versions.yaml.")
}
