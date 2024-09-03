// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpanTracerProvider(t *testing.T) {
	var s span

	got := s.TracerProvider()
	require.IsType(t, &tracerProvider{}, got)
	assert.Same(t, got.(*tracerProvider), tracerProviderInstance)
}
