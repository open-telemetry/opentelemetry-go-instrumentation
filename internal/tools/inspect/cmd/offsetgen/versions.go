// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const jsonURL = "https://go.dev/dl/?mode=json&include=all"

type goListResponse struct {
	Path     string   `json:"Path"`
	Versions []string `json:"versions"`
}

// PkgVersions returns all locally known version of module with
// moduleName.
func PkgVersions(name string) ([]*semver.Version, error) {
	command := "go list -m -json -versions " + name
	cmd := exec.Command("bash", "-c", command)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	resp := goListResponse{}
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		return nil, err
	}

	out := make([]*semver.Version, len(resp.Versions))
	for i, v := range resp.Versions {
		conv, err := semver.NewVersion(v)
		if err != nil {
			return nil, err
		}
		out[i] = conv
	}
	return out, nil
}

type goDevResponse struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// GoVersions returns all known GoVersions versions from the GoVersions package mirror at
// https://go.dev/dl/.
func GoVersions(constraints ...string) ([]*semver.Version, error) {
	res, err := http.Get(jsonURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp []goDevResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}

	var versions []*semver.Version
	for _, v := range resp {
		if v.Stable {
			stripepdV := strings.ReplaceAll(v.Version, "go", "")
			v, err := semver.NewVersion(stripepdV)
			if err != nil {
				return nil, err
			}
			versions = append(versions, v)
		}
	}

	constrained, err := constrain(versions, constraints)
	sort.SliceStable(constrained, func(i, j int) bool {
		return constrained[i].LessThan(constrained[j])
	})
	return constrained, err
}

func constrain(vers []*semver.Version, constrains []string) ([]*semver.Version, error) {
	var cnsts []*semver.Constraints
	for _, c := range constrains {
		parsed, err := semver.NewConstraint(c)
		if err != nil {
			return nil, err
		}
		cnsts = append(cnsts, parsed)
	}

	valid := func(v *semver.Version) bool {
		for _, c := range cnsts {
			if !c.Check(v) {
				return false
			}
		}
		return true
	}

	var fltr []*semver.Version
	for _, ver := range vers {
		if valid(ver) {
			fltr = append(fltr, ver)
		}
	}
	return fltr, nil
}
