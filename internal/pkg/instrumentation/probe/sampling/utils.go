// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import "errors"

var (
	errInvalidFraction = errors.New("fraction must be a positive float between 0 and 1")
	errPrecisionLoss   = errors.New("the given float cannot be represented as a fraction with the current precision")
)

// floatToNumerator converts a float to a numerator of a fraction with the given denominator.
func floatToNumerator(f float64, maxDenominator uint64) (uint64, error) {
	if f < 0 || f > 1 {
		return 0, errInvalidFraction
	}
	if f == 0 {
		return 0, nil
	}
	if f == 1 {
		return maxDenominator, nil
	}
	x := uint64(f * float64(maxDenominator))
	if x == 0 {
		return 0, errPrecisionLoss
	}
	return x, nil
}
