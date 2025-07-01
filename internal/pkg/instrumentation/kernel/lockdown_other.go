// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package kernel

func getLockdownMode() LockdownMode { return 0 }
