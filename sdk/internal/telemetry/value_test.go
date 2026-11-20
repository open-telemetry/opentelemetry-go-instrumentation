// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFloat64SpecialJSON(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		json  string
	}{
		{name: "finite", value: 1.5, json: `{"doubleValue":1.5}`},
		{name: "NaN", value: math.NaN(), json: `{"doubleValue":"NaN"}`},
		{name: "positive infinity", value: math.Inf(1), json: `{"doubleValue":"Infinity"}`},
		{name: "negative infinity", value: math.Inf(-1), json: `{"doubleValue":"-Infinity"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := Float64Value(tt.value)

			encoded, err := json.Marshal(&value)
			require.NoError(t, err)
			assert.JSONEq(t, tt.json, string(encoded))

			var decoded Value
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			if math.IsNaN(tt.value) {
				assert.True(t, math.IsNaN(decoded.AsFloat64()))
			} else {
				assert.Equal(t, tt.value, decoded.AsFloat64())
			}
		})
	}
}
