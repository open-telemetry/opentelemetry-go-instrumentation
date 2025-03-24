// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internal provides unexported types for the user package.
package internal

type User struct {
	ID    int
	Name  string
	Quota int
}
