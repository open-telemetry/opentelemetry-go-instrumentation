// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"
)

var validTraceID = trace.TraceID{0x1}

func TestIDGenerator(t *testing.T) {
	ctx := context.Background()
	tID, sID := trace.TraceID{}, trace.SpanID{}
	t.Run("BackgroundContext", testIDGenerator(ctx, tID, sID))

	// Fail gracefully.
	ctx = context.WithValue(ctx, eBPFEventKey, true)
	t.Run("InvalidContext", testIDGenerator(ctx, tID, sID))

	tID, sID = validTraceID, trace.SpanID{0x1}
	span := ptrace.NewSpan()
	span.SetTraceID(pcommon.TraceID(tID))
	span.SetSpanID(pcommon.SpanID(sID))
	ctx = contextWithSpan(ctx, span)
	t.Run("Valid", testIDGenerator(ctx, tID, sID))
}

func testIDGenerator(ctx context.Context, wantTID trace.TraceID, wantSID trace.SpanID) func(*testing.T) {
	gen := newIDGenerator()

	return func(t *testing.T) {
		tid, sid := gen.NewIDs(ctx)
		assert.Equal(t, wantTID, tid)
		assert.Equal(t, wantSID, sid)

		sid = gen.NewSpanID(ctx, validTraceID)
		assert.Equal(t, wantSID, sid)
	}
}
