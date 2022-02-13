package process

import (
	"errors"
	"os"
)

const (
	ExePathEnvVar = "OTEL_TARGET_EXE"
)

type TargetArgs struct {
	ExePath string
}

func (t *TargetArgs) Validate() error {
	if t.ExePath == "" {
		return errors.New("target binary path not specified")
	}

	return nil
}

func ParseTargetArgs() *TargetArgs {
	result := &TargetArgs{}

	val, exists := os.LookupEnv(ExePathEnvVar)
	if exists {
		result.ExePath = val
	}

	return result
}

