// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"strconv"
)

// protoInt64 represents the protobuf encoding of integers which can be either
// strings or integers.
type protoInt64 int64

// Int64 returns the protoInt64 as an int64.
func (i *protoInt64) Int64() int64 { return int64(*i) }

// UnmarshalJSON decodes both strings and integers.
func (i *protoInt64) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		parsedInt, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		*i = protoInt64(parsedInt)
	} else {
		var parsedInt int64
		if err := json.Unmarshal(data, &parsedInt); err != nil {
			return err
		}
		*i = protoInt64(parsedInt)
	}
	return nil
}
