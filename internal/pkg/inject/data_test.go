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
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestGetOffset(t *testing.T) {
	dataFile := `{
	"data" : {
		"struct_1" : { 
			"field_1" : [{
				"versions": {
					"oldest": "1.7.0",
					"newest": "1.10.0"
				},
				"offsets": [
					{ "offset": 2, "since": "1.7.0" },
					{ "offset": 3, "since": "1.7.3" },
					{ "offset": 4, "since": "1.8.0" },
					{ "offset": 5, "since": "1.8.1" },
					{ "offset": 6, "since": "1.8.2" },
					{ "offset": 7, "since": "1.9.0" }
				]
			}],
			"field_2" : [{
				"versions": {
					"oldest": "1.0.0",
					"newest": "1.2.0"
				},
				"offsets": [
					{ "offset": 200, "since": "1.0.0" }
				]
			}]
		},
		"struct_2" : { 
			"field_1" : [{
				"versions": {
					"oldest": "1.0.0",
					"newest": "1.15.0"
				},
				"offsets": [
					{ "offset": 1000, "since": "1.0.0" }
				]
			}]
		}
	}
}`
	data := &TrackedOffsets{}
	err := json.Unmarshal([]byte(dataFile), data)
	require.NoError(t, err)

	testNoOffset(t, data, "struct_1", "field_1", "1.5.1")
	testNoOffset(t, data, "struct_1", "field_1", "1.6.0")
	testHasOffset(t, data, "struct_1", "field_1", "1.7.0", 2)
	testHasOffset(t, data, "struct_1", "field_1", "1.7.1", 2)
	testHasOffset(t, data, "struct_1", "field_1", "1.7.2", 2)
	testHasOffset(t, data, "struct_1", "field_1", "1.7.3", 3)
	testHasOffset(t, data, "struct_1", "field_1", "1.7.4", 3)
	testHasOffset(t, data, "struct_1", "field_1", "1.8.0", 4)
	testHasOffset(t, data, "struct_1", "field_1", "1.8.1", 5)
	testHasOffset(t, data, "struct_1", "field_1", "1.8.2", 6)
	testHasOffset(t, data, "struct_1", "field_1", "1.8.3", 6)
	testHasOffset(t, data, "struct_1", "field_1", "1.8.100", 6)
	testHasOffset(t, data, "struct_1", "field_1", "1.9.0", 7)
	testHasOffset(t, data, "struct_1", "field_1", "1.9.1", 7)
	testHasOffset(t, data, "struct_1", "field_1", "1.10.0", 7)
	testNoOffset(t, data, "struct_1", "field_1", "1.10.1")
	testNoOffset(t, data, "struct_1", "field_1", "1.11.0")
	testNoOffset(t, data, "struct_1", "field_1", "2.0.0")

	// Handles multiple fields.
	testNoOffset(t, data, "struct_1", "field_2", "0.1.0")
	testHasOffset(t, data, "struct_1", "field_2", "1.0.0", 200)
	testHasOffset(t, data, "struct_1", "field_2", "1.0.1", 200)
	testHasOffset(t, data, "struct_1", "field_2", "1.1.0", 200)
	testHasOffset(t, data, "struct_1", "field_2", "1.2.0", 200)
	testNoOffset(t, data, "struct_1", "field_2", "1.3.0")

	// No field_3 entry.
	testNoOffset(t, data, "struct_1", "field_3", "1.0.0")

	// Supports Multiple structs.
	testHasOffset(t, data, "struct_2", "field_1", "1.1.0", 1000)

	// Handles missing structs.
	testNoOffset(t, data, "struct_3", "field_1", "1.0.0")
}

func TestGetOffsetFromTracked(t *testing.T) {
	data := &TrackedOffsets{}
	err := json.Unmarshal([]byte(offsetsData), data)
	require.NoError(t, err)

	testHasOffset(t, data, "golang.org/x/net/http2.FrameHeader", "StreamID", "1.38.5", 8)
	testHasOffset(t, data, "google.golang.org/grpc/internal/transport.Stream", "method", "1.14.9", 80)
	testHasOffset(t, data, "google.golang.org/grpc/internal/transport.Stream", "method", "1.15.0", 64)
	testHasOffset(t, data, "google.golang.org/grpc/internal/transport.Stream", "method", "1.37.1", 80)
	testHasOffset(t, data, "runtime.g", "goid", "1.20.0", 152)
	testHasOffset(t, data, "net/http.Request", "URL", "1.20.2", 16)
	testHasOffset(t, data, "net/url.URL", "Path", "1.20.2", 56)

	testNoOffset(t, data, "net/url.URL", "Path", "1.8.0")
	testNoOffset(t, data, "net/url.URL", "Foo", "1.15.0")
}

func testHasOffset(t *testing.T, data *TrackedOffsets, strct, field, ver string, want uint64) {
	t.Helper()

	v, err := version.NewVersion(ver)
	require.NoError(t, err)

	got, ok := data.GetOffset(strct, field, v)
	if assert.Truef(t, ok, "missing offset: %s.%s %s", strct, field, ver) {
		assert.Equalf(t, want, got, "invalid offset: %s.%s %s", strct, field, ver)
	}
}

func testNoOffset(t *testing.T, data *TrackedOffsets, strct, field, ver string) {
	t.Helper()

	v, err := version.NewVersion(ver)
	require.NoError(t, err)

	o, ok := data.GetOffset(strct, field, v)
	assert.Falsef(t, ok, "has offset, but should not: %s.%s %s: %d", strct, field, ver, o)
}
