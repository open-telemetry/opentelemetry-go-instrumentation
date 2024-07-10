// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ExePathEnvVar is the environment variable key whose value points to the
// instrumented executable.
const ExePathEnvVar = "OTEL_GO_AUTO_TARGET_EXE"

// TargetArgs are the binary target information.
type TargetArgs struct {
	ExePath string
	Pid     int
}

// Validate validates t and returns an error if not valid.
func (t *TargetArgs) Validate() error {
	if t.Pid != 0 {
		return validatePID(t.Pid)
	}
	if t.ExePath == "" {
		return errors.New("target binary path not specified, please specify " + ExePathEnvVar + " env variable")
	}

	return nil
}

func validatePID(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("can't find process with pid %d", pid)
	}
	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return fmt.Errorf("process with pid %d does not exist", pid)
	}
	return nil
}
