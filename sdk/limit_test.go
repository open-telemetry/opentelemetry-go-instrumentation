// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpanAttrValLenLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.AttrValueLen },
		-1,
		"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT",
		"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT",
	)
}

func TestSpanAttrsLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.Attrs },
		128,
		"OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT",
		"OTEL_ATTRIBUTE_COUNT_LIMIT",
	)
}

func TestSpanEventsLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.Events },
		128,
		"OTEL_SPAN_EVENT_COUNT_LIMIT",
	)
}

func TestSpanLinksLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.Links },
		128,
		"OTEL_SPAN_LINK_COUNT_LIMIT",
	)
}

func TestSpanEventAttrsLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.EventAttrs },
		128,
		"OTEL_EVENT_ATTRIBUTE_COUNT_LIMIT",
	)
}

func TestSpanLinkAttrsLimit(t *testing.T) {
	testLimit(
		t,
		func(sl spanLimits) int { return sl.LinkAttrs },
		128,
		"OTEL_LINK_ATTRIBUTE_COUNT_LIMIT",
	)
}

func testLimit(t *testing.T, f func(spanLimits) int, zero int, keys ...string) {
	t.Helper()

	t.Run("Default", func(t *testing.T) {
		assert.Equal(t, zero, f(newSpanLimits()))
	})

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "43")
			assert.Equal(t, 43, f(newSpanLimits()))
		})
	}
}
