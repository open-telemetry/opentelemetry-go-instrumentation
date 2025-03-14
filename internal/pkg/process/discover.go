// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"log/slog"
)

// Analyzer is used to find actively running processes.
type Analyzer struct {
	id     ID
	logger *slog.Logger
}

// NewAnalyzer returns a new [ProcessAnalyzer].
func NewAnalyzer(logger *slog.Logger, id ID) *Analyzer {
	return &Analyzer{id: id, logger: logger}
}
