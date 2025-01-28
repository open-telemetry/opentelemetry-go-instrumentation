// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"math"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var bootTimeOffset = func() int64 {
	o, err := estimateBootTimeOffset()
	if err != nil {
		panic(err)
	}
	return o
}()

// BootOffsetToTimestamp returns the [pcommon.Timestamp] that is nsec number of
// nanoseconds after the estimated boot time of the system.
func BootOffsetToTimestamp(nsec uint64) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(BootOffsetToTime(nsec))
}

// BootOffsetToTime returns the timestamp that is nsec number of nanoseconds
// after the estimated boot time of the system.
func BootOffsetToTime(nsec uint64) time.Time {
	if nsec > math.MaxInt64 {
		nsec = math.MaxInt64
	}
	return time.Unix(0, bootTimeOffset+int64(nsec)) // nolint: gosec  // Bound checked.
}

// TimeToBootOffset returns the number of nanoseconds after the estimated boot
// time of the process that the timestamp represent.
func TimeToBootOffset(timestamp time.Time) uint64 {
	nsec := timestamp.UnixNano() - bootTimeOffset
	if nsec < 0 {
		return 0
	}
	return uint64(nsec)
}
