// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"context"
	"debug/buildinfo"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
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
	logger    *slog.Logger
	BuildInfo *buildinfo.BuildInfo
}

// NewAnalyzer returns a new [ProcessAnalyzer].
func NewAnalyzer(logger *slog.Logger) *Analyzer {
	return &Analyzer{logger: logger}
}

// DiscoverProcessID searches for the target as an actively running process,
// returning its PID if found.
func (a *Analyzer) DiscoverProcessID(ctx context.Context, target *TargetArgs) (int, error) {
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
		case <-ctx.Done():
			a.logger.Debug("stopping process id discovery due to kill signal")
			return 0, ErrInterrupted
		case <-t.C:
			pid, err := a.findProcessID(target, proc)
			if err == nil {
				a.logger.Info("found process", "pid", pid)
				return pid, nil
			}
			if errors.Is(err, ErrProcessNotFound) {
				a.logger.Debug("process not found yet, trying again soon", "exe_path", target.ExePath)
			} else {
				a.logger.Error("error while searching for process", "error", err, "exe_path", target.ExePath)
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
