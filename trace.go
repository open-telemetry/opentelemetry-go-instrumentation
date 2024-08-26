// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"go.opentelemetry.io/auto/internal/sdk"
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider returns a [trace.TracerProvider] that will be
// auto-instrumented by an [Instrumentation]. All trace telemetry produced will
// be processed and handled by the [Instrumentation] that auto-instruments the
// target process using the returned [trace.TracerProvider].
//
// If no [Instrumentation] is running that targets the process using the
// returned [trace.TracerProvider], that TracerProvider will perform no
// operations. No telemetry will be gathered, processed, nor exported.
func TracerProvider() trace.TracerProvider { return sdk.GetTracerProvider() }
