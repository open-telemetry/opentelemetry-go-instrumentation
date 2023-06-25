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

	"github.com/google/gops/goprocess"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/auto/pkg/errors"
	"go.opentelemetry.io/auto/pkg/instrumentors"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/opentelemetry"
	"go.opentelemetry.io/auto/pkg/process"
)

// Interface defines orchestrator interface.
type Interface interface {
	Run() error
}
type impl struct {
	ctx             context.Context
	analyzer        *process.Analyzer
	targetArgs      *process.TargetArgs
	processch       chan *goprocess.P
	deadProcess     chan int
	managers        map[int]*instrumentors.Manager
	exporter        sdktrace.SpanExporter
	ignoreProcesses map[string]interface{}
	pidTicker       <-chan time.Time
}

// New creates a new Implementation of orchestrator Interface.
func New(
	ctx context.Context,
	targetArgs *process.TargetArgs,
	exporter sdktrace.SpanExporter,
) (Interface, error) {
	// TODO read from env var
	ignoreProcesses := make(map[string]interface{})
	ignoreProcesses["dockerd"] = nil
	ignoreProcesses["containerd"] = nil
	ignoreProcesses["gopls"] = nil
	ignoreProcesses["docker-proxy"] = nil
	ignoreProcesses["otel-go-instrumentation"] = nil
	ignoreProcesses["gops"] = nil
	ignoreProcesses["containerd-shim-runc-v2"] = nil

	return &impl{
		ctx:             ctx,
		analyzer:        process.NewAnalyzer(),
		targetArgs:      targetArgs,
		exporter:        exporter,
		ignoreProcesses: ignoreProcesses,
		processch:       make(chan *goprocess.P, 10),
		deadProcess:     make(chan int, 10),
		managers:        make(map[int]*instrumentors.Manager),
		pidTicker:       time.NewTicker(2 * time.Second).C,
	}, nil
}

func (i *impl) Run() error {
	go i.findProcess()
	for {
		select {
		case <-i.ctx.Done():
			log.Logger.Info("Got SIGTERM cleaning up")

			for _, m := range i.managers {
				m.Close()
			}

			close(i.deadProcess)
			close(i.processch)

			return nil
		case d := <-i.deadProcess:
			log.Logger.Info("process died cleaning up", "pid", d)
			if m, ok := i.managers[d]; ok {
				m.Close()
			}
			delete(i.managers, d)

		case p := <-i.processch:
			serviceName := p.Exec
			if i.targetArgs != nil && i.targetArgs.ServiceName != "" {
				serviceName = i.targetArgs.ServiceName
			}

			log.Logger.V(0).Info(
				"Add auto instrumetors",
				"pid",
				p.PID,
				"serviceName",
				serviceName,
				"Exec",
				p.Exec,
			)
			controller, err := opentelemetry.NewController(i.ctx, serviceName, i.exporter)
			if err != nil {
				log.Logger.Error(err, "error creating opentelemetry controller")
				continue
			}

			instManager, err := instrumentors.NewManager(controller)
			if err != nil {
				log.Logger.Error(err, "error creating instrumetors manager")
				continue
			}

			targetDetails, err := i.analyzer.Analyze(p.PID, instManager.GetRelevantFuncs())
			if err != nil {
				log.Logger.Error(err, "error while analyzing target process")
				continue
			}
			log.Logger.V(0).Info("target process analysis completed", "pid", targetDetails.PID,
				"go_version", targetDetails.GoVersion, "dependencies", targetDetails.Libraries,
				"total_functions_found", len(targetDetails.Functions))
			i.managers[targetDetails.PID] = instManager
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

func (i *impl) findProcess() {
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-i.pidTicker:

			prs := goprocess.FindAll()
			pmap := make(map[int]goprocess.P)
			for _, p := range prs {
				if _, ok := i.ignoreProcesses[p.Exec]; ok {
					log.Logger.V(1).Info("ignoring process", "process", p)
					continue
				}
				// filter pids based on targetArgs
				if i.targetArgs != nil && !strings.Contains(p.Path, i.targetArgs.ExePath) {
					continue
				}
				pmap[p.PID] = p
			}

			if len(pmap) == 0 {
				for pid := range i.managers {
					i.deadProcess <- pid
				}

				log.Logger.V(0).
					Info("No go process not found yet, trying again in 2 seconds")
				continue
			}

			for pid := range i.managers {
				if _, ok := pmap[pid]; !ok {
					i.deadProcess <- pid
				}
			}
			for p, pp := range pmap {
				if _, ok := i.managers[p]; !ok {
					i.processch <- &pp
				}
			}
		}
	}
}
