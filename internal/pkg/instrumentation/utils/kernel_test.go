// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBootOffsetConversion(t *testing.T) {
	var sec, nsec int64 = 1e3, 9328646329 + bootTimeOffset

	timestamp := time.Unix(sec, nsec)
	t.Logf("timestamp: %v", timestamp)

	offset := uint64((sec * 1e9) + nsec - bootTimeOffset)
	t.Logf("offset: %d", offset)

	assert.Equal(t, offset, TimeToBootOffset(timestamp), "TimeToBootOffset")
	assert.Equal(t, timestamp, BootOffsetToTime(offset), "BootOffsetToTime")
}
