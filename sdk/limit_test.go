// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpanLimit(t *testing.T) {
	tests := []struct {
		name string
		get  func(spanLimits) int
		zero int
		keys []string
	}{
		{
			name: "AttributeValueLengthLimit",
			get:  func(sl spanLimits) int { return sl.AttrValueLen },
			zero: -1,
			keys: []string{
				"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT",
				"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT",
			},
		},
		{
			name: "AttributeCountLimit",
			get:  func(sl spanLimits) int { return sl.Attrs },
			zero: 128,
			keys: []string{
				"OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT",
				"OTEL_ATTRIBUTE_COUNT_LIMIT",
			},
		},
		{
			name: "EventCountLimit",
			get:  func(sl spanLimits) int { return sl.Events },
			zero: 128,
			keys: []string{"OTEL_SPAN_EVENT_COUNT_LIMIT"},
		},
		{
			name: "EventAttributeCountLimit",
			get:  func(sl spanLimits) int { return sl.EventAttrs },
			zero: 128,
			keys: []string{"OTEL_EVENT_ATTRIBUTE_COUNT_LIMIT"},
		},
		{
			name: "LinkCountLimit",
			get:  func(sl spanLimits) int { return sl.Links },
			zero: 128,
			keys: []string{"OTEL_SPAN_LINK_COUNT_LIMIT"},
		},
		{
			name: "LinkAttributeCountLimit",
			get:  func(sl spanLimits) int { return sl.LinkAttrs },
			zero: 128,
			keys: []string{"OTEL_LINK_ATTRIBUTE_COUNT_LIMIT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("Default", func(t *testing.T) {
				assert.Equal(t, test.zero, test.get(newSpanLimits()))
			})

			t.Run("ValidValue", func(t *testing.T) {
				for _, key := range test.keys {
					t.Run(key, func(t *testing.T) {
						t.Setenv(key, "43")
						assert.Equal(t, 43, test.get(newSpanLimits()))
					})
				}
			})

			t.Run("InvalidValue", func(t *testing.T) {
				for _, key := range test.keys {
					t.Run(key, func(t *testing.T) {
						t.Setenv(key, "invalid int value.")
						assert.Equal(t, test.zero, test.get(newSpanLimits()))
					})
				}
			})
		})
	}
}
