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

package binary

import "go.opentelemetry.io/auto/internal/pkg/log"

// Func represents a function target.
type Func struct {
	Name          string
	Offset        uint64
	ReturnOffsets []uint64
}

func logFoundFunction(name string, offset uint64, returns []uint64) {
	log.Logger.V(0).Info("found relevant function for instrumentation",
		"function", name,
		"start", offset,
		"returns", returns)
}
