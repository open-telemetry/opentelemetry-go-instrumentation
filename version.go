// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import "go.opentelemetry.io/auto/internal/pkg/instrumentation"

// Version is the current release version of OpenTelemetry Go auto-instrumentation in use.
func Version() string {
	return instrumentation.DistributionVersion
}
