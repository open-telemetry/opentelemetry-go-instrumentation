// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestContextValue(t *testing.T) {
	ctx := context.Background()

	s, ok := spanFromContext(ctx)
	assert.False(t, ok, "Background context")
	assert.Equal(t, ptrace.Span{}, s, "Background context")

	// Fail gracefully.
	ctx = context.WithValue(ctx, eBPFEventKey, true)
	s, ok = spanFromContext(ctx)
	assert.False(t, ok, "Invalid value")
	assert.Equal(t, ptrace.Span{}, s, "Invalid value")

	want := ptrace.NewSpan()
	want.SetName("name")
	ctx = contextWithSpan(ctx, want)
	s, ok = spanFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, want, s)
}
