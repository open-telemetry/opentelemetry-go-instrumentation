// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build tools
// +build tools

package tools // import "go.opentelemetry.io/auto/internal/tools"

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/go-licenses/v2"
	_ "go.opentelemetry.io/build-tools/crosslink"
	_ "go.opentelemetry.io/build-tools/dbotconf"
	_ "go.opentelemetry.io/build-tools/multimod"
)
