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

package structfield

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	v12  = version.Must(version.NewVersion("1.2"))
	v120 = version.Must(version.NewVersion("1.2.0"))
	v121 = version.Must(version.NewVersion("1.2.1"))
	v130 = version.Must(version.NewVersion("1.3.0"))
)

func TestOffsets(t *testing.T) {
	var o Offsets

	off, ok := o.Get(v120)
	assert.False(t, ok, "empty offsets found value")
	assert.Equal(t, uint64(0), off, "empty offset value")

	o.Put(v120, 1)
	o.Put(v121, 2)

	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0")
	assert.Equal(t, uint64(1), off, "invalid value for 1.2.0")

	off, ok = o.Get(v12)
	assert.True(t, ok, "did not get 1.2")
	assert.Equal(t, uint64(1), off, "invalid value for 1.2")

	off, ok = o.Get(v121)
	assert.True(t, ok, "did not get 1.2.1")
	assert.Equal(t, uint64(2), off, "invalid value for 1.2.1")

	o.Put(v120, 1)
	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0 after reset")
	assert.Equal(t, uint64(1), off, "invalid reset value for 1.2.0")

	o.Put(v120, 2)
	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0 after update")
	assert.Equal(t, uint64(2), off, "invalid update value for 1.2.0")
}

var index = &Index{
	data: map[ID]*Offsets{
		NewID("net/http", "Request", "Method"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: 1, version: v120},
				newVerKey(v121): {offset: 1, version: v121},
				newVerKey(v130): {offset: 1, version: v130},
			},
		},
		NewID("net/http", "Request", "URL"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: 0, version: v120},
				newVerKey(v121): {offset: 1, version: v121},
				newVerKey(v130): {offset: 2, version: v130},
			},
		},
		NewID("net/http", "Response", "Status"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: 0, version: v120},
			},
		},
		NewID("google.golang.org/grpc", "ClientConn", "target"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: 0, version: v120},
			},
		},
	},
}

func TestIndexMarshalJSON(t *testing.T) {
	const prefix, indent = "", "  "

	raw, err := os.ReadFile("testdata/offsets.json")
	require.NoError(t, err)

	// Don't compare whitespace.
	var buf bytes.Buffer
	require.NoError(t, json.Indent(&buf, raw, prefix, indent))
	want := strings.TrimSpace(buf.String())

	got, err := json.MarshalIndent(index, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

func TestIndexUnmarshalJSON(t *testing.T) {
	f, err := os.Open("testdata/offsets.json")
	require.NoError(t, err)

	var got Index
	require.NoError(t, json.NewDecoder(f).Decode(&got))
	assert.Equal(t, index, &got)
}
