// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package debug provides utilities for debugging instrumentation.
package debug

import (
	"os"
	"strconv"
)

// VerifierLogKey is the environment variable key used to control whether
// verifier logs should be outputted during operation.
const VerifierLogKey = "OTEL_GO_AUTO_SHOW_VERIFIER_LOG"

// VerifierLogEnabled returns if outputting the verifier logs is enabled.
func VerifierLogEnabled() bool {
	val, exists := os.LookupEnv(VerifierLogKey)
	if exists {
		boolVal, err := strconv.ParseBool(val)
		if err == nil {
			return boolVal
		}
	}

	return false
}
