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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
)

const jsonURL = "https://go.dev/dl/?mode=json&include=all"

type goListResponse struct {
	Path     string   `json:"Path"`
	Versions []string `json:"versions"`
}

// PkgVersions returns all locally known version of module with
// moduleName.
func PkgVersions(name string) ([]*version.Version, error) {
	command := fmt.Sprintf("go list -m -json -versions %s", name)
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

	out := make([]*version.Version, len(resp.Versions))
	for i, v := range resp.Versions {
		conv, err := version.NewVersion(v)
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
func GoVersions(constraints ...string) ([]*version.Version, error) {
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

	var versions []*version.Version
	for _, v := range resp {
		if v.Stable {
			stripepdV := strings.ReplaceAll(v.Version, "go", "")
			v, err := version.NewVersion(stripepdV)
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

func constrain(vers []*version.Version, constrains []string) ([]*version.Version, error) {
	var cnsts []version.Constraints
	for _, c := range constrains {
		parsed, err := version.NewConstraint(c)
		if err != nil {
			return nil, err
		}
		cnsts = append(cnsts, parsed)
	}

	valid := func(v *version.Version) bool {
		for _, c := range cnsts {
			if !c.Check(v) {
				return false
			}
		}
		return true
	}

	var fltr []*version.Version
	for _, ver := range vers {
		if valid(ver) {
			fltr = append(fltr, ver)
		}
	}
	return fltr, nil
}
