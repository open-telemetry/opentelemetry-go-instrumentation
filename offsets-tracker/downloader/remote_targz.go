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

package downloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"go.opentelemetry.io/auto/offsets-tracker/utils"
)

const (
	urlPattern = "https://go.dev/dl/go%s.linux-amd64.tar.gz"
)

// DownloadBinaryFromRemote returns the downloaded Go binary at version from
// https://go.dev/dl/.
func DownloadBinaryFromRemote(_ string, version string) (string, string, error) {
	dir, err := ioutil.TempDir("", version)
	if err != nil {
		return "", "", err
	}
	dest, err := os.Create(path.Join(dir, "go.tar.gz"))
	if err != nil {
		return "", "", err
	}
	defer dest.Close()

	resp, err := http.Get(fmt.Sprintf(urlPattern, version))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	_, err = io.Copy(dest, resp.Body)
	if err != nil {
		return "", "", err
	}

	_, _, err = utils.RunCommand("tar -xf go.tar.gz -C .", dir)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/go/bin/go", dir), dir, nil
}
