// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFloat64ToNumerator(t *testing.T) {
	tests := []struct {
		name              string
		f                 float64
		maxDenominator    uint64
		expectedNumerator uint64
		expectedError     error
	}{
		{
			name:              "50 = 0.5 * 100",
			f:                 0.5,
			maxDenominator:    100,
			expectedNumerator: 50,
			expectedError:     nil,
		},
		{
			name:              "invalid input",
			f:                 1.5,
			maxDenominator:    100,
			expectedNumerator: 0,
			expectedError:     errInvalidFraction,
		},
		{
			name:              "1 = 0.01 * 100",
			f:                 0.01,
			maxDenominator:    100,
			expectedNumerator: 1,
			expectedError:     nil,
		},
		{
			name:           "precision loss",
			f:              0.00001,
			maxDenominator: 100,
			expectedError:  errPrecisionLoss,
		},
		{
			name:              "0 = 0 * 100",
			f:                 0,
			maxDenominator:    100,
			expectedNumerator: 0,
			expectedError:     nil,
		},
		{
			name:              "100 = 1 * 100",
			f:                 1,
			maxDenominator:    100,
			expectedNumerator: 100,
			expectedError:     nil,
		},
		{
			name:              "1 = 0.00001 * 100000",
			f:                 0.00001,
			maxDenominator:    100000,
			expectedNumerator: 1,
			expectedError:     nil,
		},
		{
			name:              "99999 = 0.99999 * 100000",
			f:                 0.99999,
			maxDenominator:    100000,
			expectedNumerator: 99999,
			expectedError:     nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			numerator, err := floatToNumerator(test.f, test.maxDenominator)
			if err != nil {
				assert.ErrorIs(t, err, test.expectedError)
				return
			}
			assert.Equal(t, test.expectedNumerator, numerator)
		})
	}
}
