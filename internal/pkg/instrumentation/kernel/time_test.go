// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kernel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootOffsetConversion(t *testing.T) {
	const sec = 1e3
	nsec := 9328646329 + bootTimeOffset

	timestamp := time.Unix(sec, nsec)
	t.Logf("timestamp: %v", timestamp)

	offsetInt64 := (sec * 1e9) + nsec - bootTimeOffset
	require.GreaterOrEqual(t, offsetInt64, int64(0))
	offset := uint64(offsetInt64) // nolint: gosec  // Bounds checked.
	t.Logf("offset: %d", offset)

	assert.Equal(t, offset, TimeToBootOffset(timestamp), "TimeToBootOffset")
	assert.Equal(t, timestamp, bootOffsetToTime(offset), "BootOffsetToTime")
}
