package utils

import (
	"errors"
	"fmt"
	"github.com/cilium/ebpf"
	"os"
	"strconv"
)

const (
	ShowVerifierLogEnvVar = "OTEL_GO_AUTO_SHOW_VERIFIER_LOG"
)

func LoadEbpfObjects(spec *ebpf.CollectionSpec, to interface{}, opts *ebpf.CollectionOptions) error {
	showVerifierLogs := shouldShowVerifierLogs()
	if showVerifierLogs {
		opts.Programs.LogSize = ebpf.DefaultVerifierLogSize * 100
		opts.Programs.LogLevel = ebpf.LogLevelInstruction | ebpf.LogLevelBranch | ebpf.LogLevelStats
	}

	err := spec.LoadAndAssign(to, opts)
	if err != nil && showVerifierLogs {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			fmt.Printf("Verifier log: %+v\n", ve)
		}
	}

	return err
}

// Getting full verifier log is expensive, so we only do it if the user explicitly asks for it.
func shouldShowVerifierLogs() bool {
	val, exists := os.LookupEnv(ShowVerifierLogEnvVar)
	if exists {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			return true
		}
		return boolVal
	}

	return false
}
