// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package kernel

import "github.com/Masterminds/semver/v3"

func version() *semver.Version { return nil }
