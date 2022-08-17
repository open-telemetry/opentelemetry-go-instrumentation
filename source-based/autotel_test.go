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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/pdelewski/autotel/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testcases = map[string]string{
	"./tests/fib":        "./tests/expected/fib",
	"./tests/methods":    "./tests/expected/methods",
	"./tests/goroutines": "./tests/expected/goroutines",
	"./tests/recursion":  "./tests/expected/recursion",
	"./tests/interface":  "./tests/expected/interface",
	"./tests/package":    "./tests/expected/package",
	"./tests/selector":   "./tests/expected/selector",
}

var failures []string

func Test(t *testing.T) {

	for k, v := range testcases {
		injectAndDumpIr(k, "./...")
		files := lib.SearchFiles(k, ".go")
		expectedFiles := lib.SearchFiles(v, ".go")

		for _, file := range files {

			for _, expectedFile := range expectedFiles {
				if filepath.Base(file) == filepath.Base(expectedFile) {
					fmt.Println(file)
					fmt.Println(expectedFile)
					f1, err1 := ioutil.ReadFile(file)
					require.NoError(t, err1)
					f2, err2 := ioutil.ReadFile(expectedFile)
					require.NoError(t, err2)
					if !assert.True(t, bytes.Equal(f1, f2)) {
						fmt.Println(k)
						failures = append(failures, k)
					}
				}
			}

		}
		lib.Revert(k)
	}
	for _, f := range failures {
		fmt.Println("FAILURE : ", f)
	}
}
