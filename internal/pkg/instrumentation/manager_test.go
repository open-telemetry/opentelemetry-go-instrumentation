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

package instrumentation

import (
	"log"
	"os"
	"testing"

	"github.com/go-logr/stdr"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

func TestProbeFiltering(t *testing.T) {
	ver, err := version.NewVersion("1.20.0")
	assert.NoError(t, err)

	t.Run("empty target details", func(t *testing.T) {
		m := fakeManager(t)

		td := process.TargetDetails{
			PID:               1,
			Functions:         []*binary.Func{},
			GoVersion:         ver,
			Libraries:         map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 0, len(m.probes))
	})

	t.Run("only HTTP client target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			{Name: "net/http.(*Transport).roundTrip"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Libraries:         map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 1, len(m.probes)) // one function, single probe
	})

	t.Run("HTTP server and client target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			{Name: "net/http.(*Transport).roundTrip"},
			{Name: "net/http.serverHandler.ServeHTTP"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Libraries:         map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 2, len(m.probes))
	})

	t.Run("HTTP server and client dependent function only target details", func(t *testing.T) {
		m := fakeManager(t)

		httpFuncs := []*binary.Func{
			// writeSubset depends on "net/http.(*Transport).roundTrip", it should be ignored without roundTrip
			{Name: "net/http.Header.writeSubset"},
			{Name: "net/http.serverHandler.ServeHTTP"},
		}

		td := process.TargetDetails{
			PID:               1,
			Functions:         httpFuncs,
			GoVersion:         ver,
			Libraries:         map[string]*version.Version{},
			AllocationDetails: nil,
		}
		m.FilterUnusedProbes(&td)
		assert.Equal(t, 1, len(m.probes))
	})
}

func fakeManager(t *testing.T) *Manager {
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	logger = logger.WithName("Instrumentation")

	m, err := NewManager(logger, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, m)

	return m
}
