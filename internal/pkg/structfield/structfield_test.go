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
	v110 = version.Must(version.NewVersion("1.1.0"))
	v12  = version.Must(version.NewVersion("1.2"))
	v120 = version.Must(version.NewVersion("1.2.0"))
	v121 = version.Must(version.NewVersion("1.2.1"))
	v130 = version.Must(version.NewVersion("1.3.0"))
)

func TestOffsets(t *testing.T) {
	var o Offsets

	off, ok := o.Get(v120)
	assert.False(t, ok, "empty offsets found value")
	assert.Equal(t, OffsetKey{Offset: 0, Valid: false}, off, "empty offset value")

	o.Put(v120, OffsetKey{Offset: 1, Valid: true})
	o.Put(v121, OffsetKey{Offset: 2, Valid: true})
	o.Put(v130, OffsetKey{Offset: 0, Valid: false})

	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0")
	assert.Equal(t, OffsetKey{Offset: 1, Valid: true}, off, "invalid value for 1.2.0")

	off, ok = o.Get(v12)
	assert.True(t, ok, "did not get 1.2")
	assert.Equal(t, OffsetKey{Offset: 1, Valid: true}, off, "invalid value for 1.2")

	off, ok = o.Get(v130)
	assert.True(t, ok, "did not get 1.3.0")
	assert.Equal(t, OffsetKey{Offset: 0, Valid: false}, off, "invalid value for 1.3.0")

	_, ok = o.Get(v110)
	assert.False(t, ok, "found value for 1.1.0")

	off, ok = o.Get(v121)
	assert.True(t, ok, "did not get 1.2.1")
	assert.Equal(t, OffsetKey{Offset: 2, Valid: true}, off, "invalid value for 1.2.1")

	o.Put(v120, OffsetKey{Offset: 1, Valid: true})
	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0 after reset")
	assert.Equal(t, OffsetKey{Offset: 1, Valid: true}, off, "invalid reset value for 1.2.0")

	o.Put(v120, OffsetKey{Offset: 2, Valid: true})
	off, ok = o.Get(v120)
	assert.True(t, ok, "did not get 1.2.0 after update")
	assert.Equal(t, OffsetKey{Offset: 2, Valid: true}, off, "invalid update value for 1.2.0")
}

var index = &Index{
	data: map[ID]*Offsets{
		NewID("std", "net/http", "Request", "Method"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: OffsetKey{Offset: 1, Valid: true}, version: v120},
				newVerKey(v121): {offset: OffsetKey{Offset: 1, Valid: true}, version: v121},
				newVerKey(v130): {offset: OffsetKey{Offset: 1, Valid: true}, version: v130},
			},
			uo: uniqueOffset{value: 1, valid: true},
		},
		NewID("std", "net/http", "Request", "URL"): {
			values: map[verKey]offsetVersion{
				newVerKey(v110): {offset: OffsetKey{Offset: 0, Valid: false}, version: v110},
				newVerKey(v120): {offset: OffsetKey{Offset: 0, Valid: true}, version: v120},
				newVerKey(v121): {offset: OffsetKey{Offset: 1, Valid: true}, version: v121},
				newVerKey(v130): {offset: OffsetKey{Offset: 2, Valid: true}, version: v130},
			},
			uo: uniqueOffset{value: 0, valid: false},
		},
		NewID("std", "net/http", "Response", "Status"): {
			values: map[verKey]offsetVersion{
				newVerKey(v110): {offset: OffsetKey{Offset: 0, Valid: false}, version: v110},
				newVerKey(v120): {offset: OffsetKey{Offset: 0, Valid: true}, version: v120},
			},
			uo: uniqueOffset{value: 0, valid: false},
		},
		NewID("google.golang.org/grpc", "google.golang.org/grpc", "ClientConn", "target"): {
			values: map[verKey]offsetVersion{
				newVerKey(v120): {offset: OffsetKey{Offset: 0, Valid: true}, version: v120},
			},
			uo: uniqueOffset{value: 0, valid: true},
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
