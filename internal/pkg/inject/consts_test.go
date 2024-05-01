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

package inject

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

func TestWithRegistersABI(t *testing.T) {
	opts := []Option{WithRegistersABI(true)}
	got, err := newConsts(opts)
	require.NoError(t, err)
	require.Contains(t, got, keyIsRegistersABI)

	v := got[keyIsRegistersABI]
	require.IsType(t, *(new(bool)), v)
	assert.True(t, v.(bool))
}

func TestWithAllocationDetails(t *testing.T) {
	const start, end, nCPU uint64 = 1, 2, 3
	details := process.AllocationDetails{
		StartAddr: start,
		EndAddr:   end,
		NumCPU:    nCPU,
	}

	opts := []Option{WithAllocationDetails(details)}
	got, err := newConsts(opts)
	require.NoError(t, err)
	require.Contains(t, got, keyTotalCPUs)
	require.Contains(t, got, keyStartAddr)
	require.Contains(t, got, keyEndAddr)

	v := got[keyTotalCPUs]
	require.IsType(t, *(new(uint64)), v)
	assert.Equal(t, nCPU, v.(uint64))

	v = got[keyStartAddr]
	require.IsType(t, *(new(uint64)), v)
	assert.Equal(t, start, v.(uint64))

	v = got[keyEndAddr]
	require.IsType(t, *(new(uint64)), v)
	assert.Equal(t, end, v.(uint64))
}

func TestWithOffset(t *testing.T) {
	v10, err := version.NewVersion("1.0")
	require.NoError(t, err)
	v18, err := version.NewVersion("1.8")
	require.NoError(t, err)

	const off uint64 = 1
	id := structfield.NewID("std", "net/http", "Request", "Method")

	origOff := offsets
	t.Cleanup(func() { offsets = origOff })
	offsets = structfield.NewIndex()
	offsets.PutOffset(id, v10, off, true)
	offsets.PutOffset(id, v18, off, true)

	const name = "test_name"
	opts := []Option{WithOffset(name, id, v10)}
	got, err := newConsts(opts)
	require.NoError(t, err)
	require.Contains(t, got, name)

	v := got[name]
	require.IsType(t, *(new(uint64)), v)
	assert.Equal(t, off, v.(uint64))

	// Failed look-ups need to be returned as an error.
	id.Struct = id.Struct + "Alt"
	opts = []Option{WithOffset(name, id, v10)}
	_, err = newConsts(opts)
	assert.ErrorIs(t, err, errNotFound)
}
