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

package process

import (
	"debug/elf"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/auto/pkg/log"
)

// Analyzer is used to find actively running processes.
type Analyzer struct {
	done            chan bool
	pidTickerChan   <-chan time.Time
	ignoreProcesses map[string]any
}

// NewAnalyzer returns a new [ProcessAnalyzer].
func NewAnalyzer() *Analyzer {
	// TODO read from env var
	ignoreProcesses := make(map[string]any)
	ignoreProcesses["dockerd"] = nil
	ignoreProcesses["containerd"] = nil
	ignoreProcesses["gopls"] = nil
	ignoreProcesses["docker-proxy"] = nil
	ignoreProcesses["otel-go-instrumentation"] = nil
	ignoreProcesses["gops"] = nil
	ignoreProcesses["containerd-shim-runc-v2"] = nil
	return &Analyzer{
		done:            make(chan bool, 1),
		pidTickerChan:   time.NewTicker(2 * time.Second).C,
		ignoreProcesses: ignoreProcesses,
	}
}

// Close closes the analyzer.
func (a *Analyzer) Close() {
	a.done <- true
}

// FindAllProcesses returns all go processes by reading `/proc/`.
func (a *Analyzer) FindAllProcesses(target *TargetArgs) map[int]string {
	proc, err := os.Open("/proc")
	if err != nil {
		return nil
	}

	pids := make(map[int]string)
	for {
		dirs, err := proc.Readdir(15)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Logger.V(1).Error(err, "unable to read /proc")
			return nil
		}

		for _, di := range dirs {
			if !di.IsDir() {
				continue
			}

			dname := di.Name()
			if dname[0] < '0' || dname[0] > '9' {
				continue
			}

			pid, err := strconv.Atoi(dname)
			if err != nil {
				log.Logger.V(1).Error(err, "creating pid")
				return nil
			}

			exeFullPath, err := os.Readlink(path.Join("/proc", dname, "exe"))
			if err != nil {
				// Read link may fail if target process runs not as root
				cmdline, err := os.ReadFile(path.Join("/proc", dname, "cmdline"))
				if err != nil {
					log.Logger.V(1).Error(err, "reading cmdline")
					return nil
				}
				exeFullPath = string(cmdline)
			}

			exe := filepath.Base(exeFullPath)

			if _, ok := a.ignoreProcesses[exe]; ok {
				continue
			}
			if target != nil && !strings.Contains(string(exeFullPath), target.ExePath) {
				continue
			}
			if !a.isGo(pid) {
				continue
			}

			pids[pid] = exe
		}
	}

	return pids
}

func (a *Analyzer) isGo(pid int) bool {
	// TODO
	f, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return false
	}

	defer f.Close()
	elfF, err := elf.NewFile(f)
	if err != nil {
		log.Logger.V(1).Error(err, "creating elf file")
		return false
	}

	_, _, err = a.getModuleDetails(elfF)

	return err == nil
}
