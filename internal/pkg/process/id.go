// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	errInvalidID = errors.New("invalid ID")
	errNoID      = errors.New("no process with ID")
	errNoRunID   = errors.New("no running process with ID")
)

// ID represents a process identification number.
type ID int

// Validate returns nil if id represents a valid running process. Otherwise, an
// error is returned.
func (id ID) Validate() error {
	if id < 0 {
		return fmt.Errorf("%w: %d", errInvalidID, id)
	}

	p, err := osFindProcess(int(id))
	if err != nil {
		return fmt.Errorf("%w: %d: %w", errNoID, id, err)
	}

	err = sig(p, syscall.Signal(0))
	if err != nil {
		return fmt.Errorf("%w: %d: %w", errNoRunID, id, err)
	}
	return nil
}

var (
	osFindProcess = os.FindProcess
	sig           = sigFn
)

func sigFn(p *os.Process, s os.Signal) error { return p.Signal(s) }

func (id ID) dir() string { return procDir(id) }

var procDir = procDirFn

func procDirFn(id ID) string { return "/proc/" + strconv.Itoa(int(id)) }

// exePath returns the file path for executable link of the process ID.
func (id ID) exePath() string { return id.dir() + "/exe" }

// taskPath returns the file path for the tasks directory of the process ID.
func (id ID) taskPath() string { return id.dir() + "/task" }

// ExePath returns the resolved absolute path to the executable being run by
// the process.
func (id ID) ExePath() (string, error) {
	p, err := os.Readlink(id.exePath())
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(p) {
		return p, nil
	}

	return filepath.Abs(filepath.Join(id.dir(), p))
}

// Tasks returns the task directory contents for the process.
func (id ID) Tasks() ([]fs.DirEntry, error) {
	return os.ReadDir(id.taskPath())
}

// BuildInfo returns the Go build info of the process ID executable.
func (id ID) BuildInfo() (*buildinfo.BuildInfo, error) {
	path, err := id.ExePath()
	if err != nil {
		return nil, err
	}
	bi, err := buildinfoReadFile(path)
	if err != nil {
		return nil, err
	}

	bi.GoVersion = strings.ReplaceAll(bi.GoVersion, "go", "")
	// Trims GOEXPERIMENT version suffix if present.
	if idx := strings.Index(bi.GoVersion, " X:"); idx > 0 {
		bi.GoVersion = bi.GoVersion[:idx]
	}

	return bi, nil
}

var buildinfoReadFile = buildinfo.ReadFile
