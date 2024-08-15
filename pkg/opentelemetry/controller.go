// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import "go.opentelemetry.io/auto/pkg/probe"

type OpenTelemetryController interface {
	Trace(event *probe.Event)
}