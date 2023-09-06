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

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestGetFieldOffset(t *testing.T) {
	dataFile := `{
	"data" : {
		"struct_1" : { 
			"field_1" : {
				"offsets": [
					{ "offset": 1187, "since": "1.18.7" },
					{ "offset": 1190, "since": "1.19.0" }
				]
			}
		}
	}
}`
	injector := Injector{data: &TrackedOffsets{}}
	err := json.Unmarshal([]byte(dataFile), injector.data)
	require.NoError(t, err)

	offset, ok := injector.getFieldOffset("struct_1", "field_1", "1.19.7")
	assert.True(t, ok)
	assert.Equal(t, 1190, int(offset))
	offset, ok = injector.getFieldOffset("struct_1", "field_1", "1.19.0")
	assert.True(t, ok)
	assert.Equal(t, 1190, int(offset))
	offset, ok = injector.getFieldOffset("struct_1", "field_1", "1.18.9")
	assert.True(t, ok)
	assert.Equal(t, 1187, int(offset))
	offset, ok = injector.getFieldOffset("struct_1", "field_1", "1.17.9")
	assert.Falsef(t, ok, "found: %d", int(offset))
}

func TestGetFieldOffset_OffsetResultsJSON(t *testing.T) {
	injector := Injector{data: &TrackedOffsets{}}
	err := json.Unmarshal([]byte(offsetsData), injector.data)
	require.NoError(t, err)

	offset, ok := injector.getFieldOffset("golang.org/x/net/http2.FrameHeader", "StreamID", "1.38.5")
	assert.True(t, ok)
	assert.Equal(t, 8, int(offset))

	offset, ok = injector.getFieldOffset("google.golang.org/grpc/internal/transport.Stream", "method", "1.14.9")
	assert.True(t, ok)
	assert.Equal(t, 80, int(offset))

	offset, ok = injector.getFieldOffset("google.golang.org/grpc/internal/transport.Stream", "method", "1.15.0")
	assert.True(t, ok)
	assert.Equal(t, 64, int(offset))

	offset, ok = injector.getFieldOffset("google.golang.org/grpc/internal/transport.Stream", "method", "1.37.1")
	assert.True(t, ok)
	assert.Equal(t, 80, int(offset))

	offset, ok = injector.getFieldOffset("runtime.g", "goid", "1.20.0")
	assert.True(t, ok)
	assert.Equal(t, 152, int(offset))

	offset, ok = injector.getFieldOffset("net/http.Request", "URL", "1.20.2")
	assert.True(t, ok)
	assert.Equal(t, 16, int(offset))

	offset, ok = injector.getFieldOffset("net/url.URL", "Path", "1.20.2")
	assert.True(t, ok)
	assert.Equal(t, 56, int(offset))

	offset, ok = injector.getFieldOffset("net/url.URL", "Path", "1.8.0")
	assert.Falsef(t, ok, "found: %d", int(offset))

	offset, ok = injector.getFieldOffset("net/url.URL", "Foo", "1.15.0")
	assert.Falsef(t, ok, "found: %d", int(offset))
}
