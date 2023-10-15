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
	"strings"
	"time"

	"go.opentelemetry.io/auto/internal/pkg/errors"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors"
	"go.opentelemetry.io/auto/internal/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/opentelemetry"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

type pidServiceName struct {
	serviceName string
	pid         int
}

// New creates a new Implementation of orchestrator Service.
func New(
	ctx context.Context,
	opts ...ServiceOpt,
) (*Service, error) {
	s := Service{
		ctx:         ctx,
		analyzer:    process.NewAnalyzer(),
		processch:   make(chan *pidServiceName, 10),
		deadProcess: make(chan int, 10),
		managers:    make(map[int]*instrumentors.Manager),
		pidTicker:   time.NewTicker(2 * time.Second).C,
	}
	for _, o := range opts {
		s = o.apply(s)
	}

	s = s.applyEnv()
	return &s, nil
}

// Run manages the lifecycle of instrumentors for a go process.
func (s *Service) Run() error {
	go s.findProcess()
	for {
		select {
		case <-s.ctx.Done():

			log.Logger.Info("Got context done")
			for _, m := range s.managers {
				m.Close()
			}

			close(s.deadProcess)
			close(s.processch)
			log.Logger.Info("cleaning done, shutting down")

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
				Version:     s.version,
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

			td, err := s.analyzer.Analyze(p.pid, instManager.GetRelevantFuncs())
			if err != nil {
				log.Logger.Error(err, "error while analyzing target process")
				continue
			}
			log.Logger.V(0).Info("target process analysis completed", "pid", td.PID,
				"go_version", td.GoVersion, "dependencies", td.Libraries,
				"total_functions_found", len(td.Functions))

			allocDetails, err := process.Allocate(p.pid)
			if err != nil {
				log.Logger.Error(err, "error allocating")
				continue
			}
			td.AllocationDetails = allocDetails

			s.managers[td.PID] = instManager
			go func() {
				log.Logger.V(0).Info("invoking instrumentors")

				err = instManager.Run(td)
				if err != nil && err != errors.ErrInterrupted {
					log.Logger.Error(err, "error while running instrumentors")
				}
			}()
		}
	}
}

func (s *Service) findProcess() {
	if s.pid != 0 {
		s.processch <- &pidServiceName{
			pid:         s.pid,
			serviceName: s.serviceName,
		}
		return
	}

	if s.exePath != "" {
		pids, err := s.analyzer.FindAllProcesses(s.exePath)
		if err != nil {
			log.Logger.Error(err, "FindAllProcesses failed for exePath", "exePath", s.exePath)
			return
		}
		if len(pids) > 1 {
			log.Logger.Error(err, "Found more than one pid for exePath", "exePath", s.exePath, "no of pids", len(pids))
			return
		}
		for k := range pids {
			s.processch <- &pidServiceName{
				pid:         k,
				serviceName: s.serviceName,
			}
		}
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.pidTicker:

			pids, err := s.analyzer.FindAllProcesses("")
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
			for p, envs := range pids {
				serviceName := ""
				if v, ok := envs[envServiceNameKey]; ok {
					serviceName = v
				} else {
					if v, ok := envs[envResourceAttrKey]; ok {
						attrs := strings.TrimSpace(v)
						serviceName = serviceNameFromAttrs(attrs)
					}
				}

				// couldn't determine serviceName
				// pid wouldn't monitored
				if serviceName == "" {
					continue
				}

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
