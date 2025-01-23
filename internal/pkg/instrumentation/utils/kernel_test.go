// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

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

	offset := (sec * 1e9) + nsec - bootTimeOffset
	require.GreaterOrEqual(t, offset, 0)
	t.Logf("offset: %d", offset)

	assert.Equal(t, offset, TimeToBootOffset(timestamp), "TimeToBootOffset")
	assert.Equal(t, timestamp, BootOffsetToTime(uint64(offset)), "BootOffsetToTime") // nolint: gosec // Bound tested.
}
