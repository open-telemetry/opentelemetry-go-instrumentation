// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"debug/buildinfo"
	"log/slog"
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
