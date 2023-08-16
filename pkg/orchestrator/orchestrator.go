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

package orchestrator

import (
	"context"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/auto/pkg/errors"
	"go.opentelemetry.io/auto/pkg/instrumentors"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/opentelemetry"
	"go.opentelemetry.io/auto/pkg/process"
)

type pidServiceName struct {
	serviceName string
	pid         int
}

// Service is responsible for managing all instrumentation.
type Service struct {
	ctx         context.Context
	analyzer    *process.Analyzer
	targetArgs  *process.TargetArgs
	processch   chan *pidServiceName
	deadProcess chan int
	managers    map[int]*instrumentors.Manager
	exporter    sdktrace.SpanExporter
	pidTicker   <-chan time.Time
}

// New creates a new Implementation of orchestrator Service.
func New(
	ctx context.Context,
	targetArgs *process.TargetArgs,
	exporter sdktrace.SpanExporter,
) (*Service, error) {
	return &Service{
		ctx:         ctx,
		analyzer:    process.NewAnalyzer(),
		targetArgs:  targetArgs,
		exporter:    exporter,
		processch:   make(chan *pidServiceName, 10),
		deadProcess: make(chan int, 10),
		managers:    make(map[int]*instrumentors.Manager),
		pidTicker:   time.NewTicker(2 * time.Second).C,
	}, nil
}

// Run manages the lifecycle of instrumentors for a go process.
func (s *Service) Run() error {
	go s.findProcess()
	for {
		select {
		case <-s.ctx.Done():

			for _, m := range s.managers {
				m.Close()
			}

			close(s.deadProcess)
			close(s.processch)

			return nil
		case d := <-s.deadProcess:
			log.Logger.Info("process died cleaning up", "pid", d)
			if m, ok := s.managers[d]; ok {
				m.Close()
			}
			err := bpffs.Cleanup(&process.TargetDetails{
				PID: d,
			})
			if err != nil {
				log.Logger.V(0).Error(err, "unable to clean bpffs", "pid", d)
			}
			delete(s.managers, d)

		case p := <-s.processch:

			log.Logger.V(0).Info(
				"Add auto instrumetors",
				"pid",
				p.pid,
				"serviceName",
				p.serviceName,
			)
			controller, err := opentelemetry.NewController(s.ctx, opentelemetry.ControllerSetting{
				ServiceName: p.serviceName,
				Exporter:    s.exporter,
			})
			if err != nil {
				log.Logger.Error(err, "error creating opentelemetry controller")
				continue
			}

			instManager, err := instrumentors.NewManager(controller)
			if err != nil {
				log.Logger.Error(err, "error creating instrumetors manager")
				continue
			}

			targetDetails, err := s.analyzer.Analyze(p.pid, instManager.GetRelevantFuncs())
			if err != nil {
				log.Logger.Error(err, "error while analyzing target process")
				continue
			}
			log.Logger.V(0).Info("target process analysis completed", "pid", targetDetails.PID,
				"go_version", targetDetails.GoVersion, "dependencies", targetDetails.Libraries,
				"total_functions_found", len(targetDetails.Functions))
			s.managers[targetDetails.PID] = instManager
			go func() {
				log.Logger.V(0).Info("invoking instrumentors")

				err = instManager.Run(targetDetails)
				if err != nil && err != errors.ErrInterrupted {
					log.Logger.Error(err, "error while running instrumentors")
				}
			}()
		}
	}
}

func (s *Service) findProcess() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.pidTicker:

			pids, err := s.analyzer.FindAllProcesses(s.targetArgs)
			if err != nil {
				log.Logger.Error(err, "FindAllProcesses failed")
				continue
			}
			if len(pids) == 0 {
				for pid := range s.managers {
					s.deadProcess <- pid
				}

				log.Logger.V(1).
					Info("No go process not found yet, trying again in 2 seconds")
				continue
			}

			for pid := range s.managers {
				if _, ok := pids[pid]; !ok {
					s.deadProcess <- pid
				}
			}
			for p, serviceName := range pids {
				if _, ok := s.managers[p]; !ok {
					s.processch <- &pidServiceName{
						pid:         p,
						serviceName: serviceName,
					}
				}
			}
		}
	}
}
