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

package main

import (
	"fmt"
	"os"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/internal/pkg/log"
)

func main() {
	err := log.Init()
	if err != nil {
		fmt.Printf("could not init logger: %s\n", err)
		os.Exit(1)
	}

	log.Logger.V(0).Info("building OpenTelemetry Go instrumentation ...")

	r, err := auto.NewInstrumentation()
	if err != nil {
		log.Logger.Error(err, "failed to create instrumentation")
		return
	}

	log.Logger.V(0).Info("starting OpenTelemetry Go Agent ...")
	if err = r.Run(); err != nil {
		log.Logger.Error(err, "running orchestrator")
	}
}
