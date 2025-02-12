// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package structfield

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	v110 = semver.New(1, 1, 0, "", "")
	v12  = semver.New(1, 2, 0, "", "")
	v120 = semver.New(1, 2, 0, "", "")
	v121 = semver.New(1, 2, 1, "", "")
	v130 = semver.New(1, 3, 0, "", "")
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

	off, ver := o.getLatest()
	assert.Equal(t, v121, &ver.Version, "invalid version for latest")
	assert.Equal(t, OffsetKey{Offset: 2, Valid: true}, off, "invalid value for latest")

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

func TestGetLatestOffsetFromIndex(t *testing.T) {
	off, ver := index.GetLatestOffset(NewID("std", "net/http", "Request", "Method"))
	assert.Equal(t, v130, ver, "invalid version for Request.Method")
	assert.Equal(t, OffsetKey{Offset: 1, Valid: true}, off, "invalid value for Request.Method")

	off, ver = index.GetLatestOffset(NewID("std", "net/http", "Request", "URL"))
	assert.Equal(t, v130, ver, "invalid version for Request.URL")
	assert.Equal(t, OffsetKey{Offset: 2, Valid: true}, off, "invalid value for Request.URL")

	off, ver = index.GetLatestOffset(NewID("std", "net/http", "Response", "Status"))
	assert.Equal(t, v120, ver, "invalid version for Response.Status")
	assert.Equal(t, OffsetKey{Offset: 0, Valid: true}, off, "invalid value for Response.Status")

	off, ver = index.GetLatestOffset(NewID("google.golang.org/grpc", "google.golang.org/grpc", "ClientConn", "target"))
	assert.Equal(t, v120, ver, "invalid version for ClientConn.target")
	assert.Equal(t, OffsetKey{Offset: 0, Valid: true}, off, "invalid value for ClientConn.target")
}
