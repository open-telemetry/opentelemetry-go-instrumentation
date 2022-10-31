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
	"os/signal"
	"syscall"

	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/errors"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/instrumentors"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/log"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/opentelemetry"
	"github.com/open-telemetry/opentelemetry-go-instrumentation/pkg/process"
)

func main() {
	err := log.Init()
	if err != nil {
		fmt.Printf("could not init logger: %s\n", err)
		os.Exit(1)
	}

	log.Logger.V(0).Info("starting Go OpenTelemetry Agent ...")
	target := process.ParseTargetArgs()
	if err = target.Validate(); err != nil {
		log.Logger.Error(err, "invalid target args")
		return
	}

	processAnalyzer := process.NewAnalyzer()
	otelController, err := opentelemetry.NewController()
	if err != nil {
		log.Logger.Error(err, "unable to create OpenTelemetry controller")
		return
	}

	instManager, err := instrumentors.NewManager(otelController)
	if err != nil {
		log.Logger.Error(err, "error creating instrumetors manager")
		return
	}

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopper
		log.Logger.V(0).Info("Got SIGTERM, cleaning up..")
		processAnalyzer.Close()
		instManager.Close()
	}()

	pid, err := processAnalyzer.DiscoverProcessID(target)
	if err != nil {
		if err != errors.ErrInterrupted {
			log.Logger.Error(err, "error while discovering process id")
		}
		return
	}

	targetDetails, err := processAnalyzer.Analyze(pid, instManager.GetRelevantFuncs())
	if err != nil {
		log.Logger.Error(err, "error while analyzing target process")
		return
	}
	log.Logger.V(0).Info("target process analysis completed", "pid", targetDetails.PID,
		"go_version", targetDetails.GoVersion, "dependencies", targetDetails.Libraries,
		"total_functions_found", len(targetDetails.Functions))

	instManager.FilterUnusedInstrumentors(targetDetails)

	log.Logger.V(0).Info("invoking instrumentors")
	err = instManager.Run(targetDetails)
	if err != nil && err != errors.ErrInterrupted {
		log.Logger.Error(err, "error while running instrumentors")
	}
}
