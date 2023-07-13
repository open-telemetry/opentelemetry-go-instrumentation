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
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
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

			if !target.MonitorAll && strings.Contains(exeFullPath, target.ExecPath) {
				pids[pid] = target.ServiceName
				break
			}

			if !a.isGo(pid) {
				continue
			}
			envs, err := getEnvVars(pid)
			if err != nil {
				log.Logger.V(1).Error(err, "reading envs ", "pid", pid)
				continue
			}

			if v, ok := envs[otelServiceNameEnvVar]; ok {
				pids[pid] = v
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

func getEnvVars(pid int) (map[string]string, error) {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}

	// /proc/<pid>/environ file in Linux, environment variables are stored in a null byte separated format.
	envs := strings.Split(string(bytes), "\x00")
	envMap := make(map[string]string, len(envs))

	for _, s := range envs {
		split := strings.SplitN(s, "=", 2) // Split by first "=" character
		if len(split) == 2 {
			envMap[split[0]] = split[1]
		}
	}

	return envMap, nil
}
