// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"os"
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
	versionYaml, err := os.ReadFile("versions.yaml")
	if err != nil {
		t.Fatalf("Couldn't read versions.yaml file: %e", err)
		return
	}

	var versionInfo map[string]any

	err = yaml.Unmarshal(versionYaml, &versionInfo)
	if err != nil {
		t.Fatalf("Couldn't parse version.yaml: %e", err)
		return
	}

	// incredibad, but it's where the intended version is declared at the moment
	expectedVersion := versionInfo["module-sets"].(map[string]any)["auto"].(map[string]any)["version"]
	assert.Equal(t, expectedVersion, Version(), "Build version should match versions.yaml.")
}
