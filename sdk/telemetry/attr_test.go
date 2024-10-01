// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	decoded = []Attr{
		String("user", "Alice"),
		Bool("admin", true),
		Int64("floor", -2),
		Float64("impact", 0.21362),
		Slice("reports", StringValue("Bob"), StringValue("Dave")),
		Map("favorites", String("food", "hot dog"), Int("number", 13)),
		Bytes("secret", []byte("NUI4RUZGRjc5ODAzODEwM0QyNjlCNjMzODEzRkM2MEM=")),
	}

	encoded = []byte(`[{"key":"user","value":{"stringValue":"Alice"}},{"key":"admin","value":{"boolValue":true}},{"key":"floor","value":{"intValue":"-2"}},{"key":"impact","value":{"doubleValue":0.21362}},{"key":"reports","value":{"arrayValue":{"values":[{"stringValue":"Bob"},{"stringValue":"Dave"}]}}},{"key":"favorites","value":{"kvlistValue":{"values":[{"key":"food","value":{"stringValue":"hot dog"}},{"key":"number","value":{"intValue":"13"}}]}}},{"key":"secret","value":{"bytesValue":"TlVJNFJVWkdSamM1T0RBek9ERXdNMFF5TmpsQ05qTXpPREV6UmtNMk1FTT0="}}]`)
)

func TestAttrUnmarshal(t *testing.T) {
	var got []Attr
	require.NoError(t, json.Unmarshal(encoded, &got))
	assert.Equal(t, decoded, got)
}

func TestAttrMarshal(t *testing.T) {
	got, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.Equal(t, string(encoded), string(got))
}
