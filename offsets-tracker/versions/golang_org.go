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
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/go-version"
)

const (
	jsonURL = "https://go.dev/dl/?mode=json&include=all"
)

type goDevResponse struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// Go returns all known Go versions from the Go package mirror at
// https://go.dev/dl/.
func Go(constraints ...string) ([]string, error) {
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

	var versions []string
	for _, v := range resp {
		if v.Stable {
			stripepdV := strings.ReplaceAll(v.Version, "go", "")
			versions = append(versions, stripepdV)
		}
	}

	return constrain(versions, constraints)
}

func constrain(vers []string, constrains []string) ([]string, error) {
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

	var fltr []string
	for _, ver := range vers {
		v, err := version.NewVersion(ver)
		if err != nil {
			return nil, err
		}
		if valid(v) {
			fltr = append(fltr, ver)
		}
	}
	return fltr, nil
}
