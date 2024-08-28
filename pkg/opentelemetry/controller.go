// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import "go.opentelemetry.io/auto/pkg/probe"

// OpenTelemetryController is a controller that converts probe Events to
// OpenTelemetry spans and exports them.
type OpenTelemetryController interface {
	// Trace receives a probe.Event and handles conversion to OpenTelemetry
	// format and exporting.
	Trace(event *probe.Event)
}