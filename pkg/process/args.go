package process

import (
	"errors"
	"flag"
	"os"
)

const (
	ExePathEnvVar = "OTEL_TARGET_EXE"
)

type TargetArgs struct {
	ExePath string
	Stdout  bool
}

func (t *TargetArgs) Validate() error {
	if t.ExePath == "" {
		return errors.New("target binary path not specified")
	}

	return nil
}

func ParseTargetArgs() *TargetArgs {
	result := &TargetArgs{}

	printHelp := flag.Bool("help", false, "")
	otelStdout := flag.Bool("stdout", false, "if true, print otel telemetry to stdout (use for local development or debugging)")

	flag.Parse()

	if *printHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	result.Stdout = *otelStdout

	val, exists := os.LookupEnv(ExePathEnvVar)
	if exists {
		result.ExePath = val
	}

	return result
}
