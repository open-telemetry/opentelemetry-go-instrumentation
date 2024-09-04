// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"

	"go.opentelemetry.io/auto/pkg/probe"
)

// OpenTelemetryController is a controller that converts probe Events to
// OpenTelemetry spans and exports them.
type OpenTelemetryController interface {
	// Trace receives a probe.Event and handles conversion to OpenTelemetry
	// format and exporting.
	Trace(event *probe.Event)

	// Shutdown shuts down the OpenTelemetry TracerProvider.
	Shutdown(ctx context.Context) error
}