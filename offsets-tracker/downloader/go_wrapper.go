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
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path"

	"go.opentelemetry.io/auto/offsets-tracker/utils"
)

const appName = "testapp"

var (
	//go:embed wrapper/go.mod.txt
	goMod string

	//go:embed wrapper/go.modstd.txt
	goModStdLib string

	//go:embed wrapper/main.go.txt
	goMain string
)

// DownloadBinary downloads the module with modName at version.
// revive:disable-next-line:flag-parameter
func DownloadBinary(modName string, version string, isGoStandartLib bool) (string, string, error) {
	dir, err := os.MkdirTemp("", appName)
	if err != nil {
		return "", "", err
	}

	var goModContent string
	if isGoStandartLib {
		goModContent = fmt.Sprintf(goModStdLib, version)
	} else {
		goModContent = fmt.Sprintf(goMod, modName, version)
	}

	err = os.WriteFile(path.Join(dir, "go.mod"), []byte(goModContent), fs.ModePerm)
	if err != nil {
		return "", "", err
	}

	goMainContent := fmt.Sprintf(goMain, modName)
	err = os.WriteFile(path.Join(dir, "main.go"), []byte(goMainContent), fs.ModePerm)
	if err != nil {
		return "", "", err
	}

	_, _, err = utils.RunCommand("go mod tidy -compat=1.17", dir)
	if err != nil {
		return "", "", err
	}

	_, _, err = utils.RunCommand(fmt.Sprintf("GOOS=linux GOARCH=amd64 go build -o %s", appName), dir)
	if err != nil {
		return "", "", err
	}

	return path.Join(dir, appName), dir, nil
}
