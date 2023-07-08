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
	"time"

	"go.opentelemetry.io/auto/pkg/log"
)

// Analyzer is used to find actively running processes.
type Analyzer struct {
	done          chan bool
	pidTickerChan <-chan time.Time
}

// NewAnalyzer returns a new [ProcessAnalyzer].
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		done:          make(chan bool, 1),
		pidTickerChan: time.NewTicker(2 * time.Second).C,
	}
}

// Close closes the analyzer.
func (a *Analyzer) Close() {
	a.done <- true
}

// FindAllProcesses returns all go processes by reading `/proc/`.
func (a *Analyzer) FindAllProcesses(target *TargetArgs) map[int]ExeService {
	proc, err := os.Open("/proc")
	if err != nil {
		return nil
	}

	pids := make(map[int]ExeService)
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

			if _, ok := target.IgnoreProcesses[exe]; ok {
				continue
			}

			if v, ok := target.IncludeProcesses[exeFullPath]; ok {
				pids[pid] = v
			}

			// if found all included processes.
			if len(target.IncludeProcesses) > 0 && len(pids) == len(target.IncludeProcesses) {
				break
			}

			// when include is defined, don't add anything else.
			if len(target.IncludeProcesses) > 0 {
				log.Logger.V(0).Info("breaking as include is defined")
				continue
			}

			if !a.isGo(pid) {
				continue
			}

			pids[pid] = ExeService{
				ExecPath:    exe,
				ServiceName: exe,
			}
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
