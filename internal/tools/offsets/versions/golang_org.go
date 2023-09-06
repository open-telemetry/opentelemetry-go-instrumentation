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
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	jsonURL = "https://go.dev/dl/?mode=json&include=all"
)

type goDevResponse struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// FindVersionsFromGoWebsite returns all known Go versions from the Go package
// mirror at https://go.dev/dl/.
func FindVersionsFromGoWebsite() ([]string, error) {
	res, err := http.Get(jsonURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
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

	return versions, nil
}
