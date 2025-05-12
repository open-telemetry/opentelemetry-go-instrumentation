// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
)

func TestAttributes(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.Bool("key1", true),
		attribute.Int64("key2", 42),
		attribute.Int("key3", 100),
		attribute.Float64("key4", 3.14),
		attribute.String("key5", "value"),
		attribute.BoolSlice("key6", []bool{true, false, true}),
		attribute.Int64Slice("key7", []int64{1, 2, 3}),
		attribute.IntSlice("key8", []int{4, 5, 6}),
		attribute.Float64Slice("key9", []float64{1.1, 2.2, 3.3}),
		attribute.StringSlice("key10", []string{"a", "b", "c"}),
	}

	dest := pcommon.NewMap()
	Attributes(dest, attrs...)
	got := dest.AsRaw()

	assert.Equal(t, true, got["key1"], "bool")
	assert.Equal(t, int64(42), got["key2"], "int64")
	assert.Equal(t, int64(100), got["key3"], "int")
	assert.Equal(t, 3.14, got["key4"], "float64")
	assert.Equal(t, "value", got["key5"], "string")
	assert.ElementsMatch(t, []bool{true, false, true}, got["key6"], "[]bool")
	assert.ElementsMatch(t, []int64{1, 2, 3}, got["key7"], "[]int64")
	assert.ElementsMatch(t, []int64{4, 5, 6}, got["key8"], "[]int")
	assert.ElementsMatch(t, []float64{1.1, 2.2, 3.3}, got["key9"], "[]float64")
	assert.ElementsMatch(t, []string{"a", "b", "c"}, got["key10"], "[]string")
}
