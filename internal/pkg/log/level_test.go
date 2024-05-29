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

package log_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto/internal/pkg/log"
)

func TestLevel(t *testing.T) {
	testCases := []struct {
		name  string
		level log.Level
		str   string
	}{
		{
			name:  "LevelDebug",
			level: log.LevelDebug,
			str:   "debug",
		},
		{
			name:  "LevelInfo",
			level: log.LevelInfo,
			str:   "info",
		},
		{
			name:  "LevelWarn",
			level: log.LevelWarn,
			str:   "warn",
		},
		{
			name:  "LevelError",
			level: log.LevelError,
			str:   "error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.str, tc.level.String(), "string does not match")
		})
	}
}
