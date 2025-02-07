// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probe

import "github.com/hashicorp/go-version"

// FailureMode defines the behavior that is performed when a failure occurs.
type FailureMode int

const (
	// FailureModeError will cause an error to be returned if a failure occurs.
	FailureModeError FailureMode = iota
	// FailureModeWarn will cause a warning message to be logged and allow
	// operations to continue if a failure occurs.
	FailureModeWarn
	// FailureModeIgnore will continue operations and ignore any failure that
	// occurred.
	FailureModeIgnore
)

// PackageConstraints is a versioning requirement for a package.
type PackageConstraints struct {
	// Package is the package import path that this constraint applies to.
	Package string
	// Constraints is the version constraint that is evaluated. If the
	// constraint is not satisfied, the FailureMode defines the behavior of how
	// the failure is handled.
	Constraints version.Constraints
	// FailureMode defines the behavior that is performed when the Constraint
	// is not satisfied.
	FailureMode FailureMode
}
