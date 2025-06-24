// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package kernel

func estimateBootTimeOffset() (int64, error) { return 0, nil }
