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

package opentelemetry

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/auto"
)

func TestUserAgent(t *testing.T) {
	assert.Contains(t, autoinstUserAgent, fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", auto.Version()))
}
