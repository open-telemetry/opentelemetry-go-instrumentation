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
	"errors"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

var (
	// ErrInterrupted is returned when a process was interrupted but didn't
	// fail in any other way.
	ErrInterrupted = errors.New("interrupted")

	// ErrProcessNotFound is returned when a requested process is not currently
	// running.
	ErrProcessNotFound = errors.New("process not found")
)

// Analyzer is used to find actively running processes.
type Analyzer struct {
	logger logr.Logger
	done   chan bool
}

// NewAnalyzer returns a new [ProcessAnalyzer].
func NewAnalyzer(logger logr.Logger) *Analyzer {
	return &Analyzer{
		logger: logger.WithName("Analyzer"),
		done:   make(chan bool, 1),
	}
}

// DiscoverProcessID searches for the target as an actively running process,
// returning its PID if found.
func (a *Analyzer) DiscoverProcessID(target *TargetArgs) (int, error) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()

	if target.Pid != 0 {
		return target.Pid, nil
	}

	proc, err := os.Open("/proc")
	if err != nil {
		return 0, err
	}
	defer proc.Close()

	for {
		select {
		case <-a.done:
			a.logger.Info("stopping process id discovery due to kill signal")
			return 0, ErrInterrupted
		case <-t.C:
			pid, err := a.findProcessID(target, proc)
			if err == nil {
				a.logger.Info("found process", "pid", pid)
				return pid, nil
			}
			if err == ErrProcessNotFound {
				a.logger.Info("process not found yet, trying again soon", "exe_path", target.ExePath)
			} else {
				a.logger.Error(err, "error while searching for process", "exe_path", target.ExePath)
			}

			// Reset the file offset for next iteration
			_, err = proc.Seek(0, 0)
			if err != nil {
				return 0, err
			}
		}
	}
}

func (a *Analyzer) findProcessID(target *TargetArgs, proc *os.File) (int, error) {
	for {
		dirs, err := proc.Readdir(15)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
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
				return 0, err
			}

			exeName, err := os.Readlink(path.Join("/proc", dname, "exe"))
			if err != nil {
				// Read link may fail if target process runs not as root
				cmdLine, err := os.ReadFile(path.Join("/proc", dname, "cmdline"))
				if err != nil {
					return 0, err
				}

				if strings.Contains(string(cmdLine), target.ExePath) {
					return pid, nil
				}
			} else if exeName == target.ExePath {
				return pid, nil
			}
		}
	}

	return 0, ErrProcessNotFound
}

// Close closes the analyzer.
func (a *Analyzer) Close() {
	a.done <- true
}
