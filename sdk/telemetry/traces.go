// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// A collection of Spans produced by an InstrumentationScope.
type ScopeSpans struct {
	// The instrumentation scope information for the spans in this message.
	// Semantically when InstrumentationScope isn't set, it is equivalent with
	// an empty instrumentation scope name (unknown).
	Scope *Scope `json:"scope"`
	// A list of Spans that originate from an instrumentation scope.
	Spans []*Span `json:"spans,omitempty"`
	// The Schema URL, if known. This is the identifier of the Schema that the span data
	// is recorded in. To learn more about Schema URL see
	// https://opentelemetry.io/docs/specs/otel/schemas/#schema-url
	// This schema_url applies to all spans and span events in the "spans" field.
	SchemaURL string `json:"schemaUrl,omitempty"`
}

// UnmarshalJSON decodes the OTLP formatted JSON contained in data into ss.
func (ss *ScopeSpans) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))

	t, err := decoder.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('{') {
		return errors.New("invalid ScopeSpans type")
	}

	for decoder.More() {
		keyIface, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Empty.
				return nil
			}
			return err
		}

		key, ok := keyIface.(string)
		if !ok {
			return fmt.Errorf("invalid ScopeSpans field: %#v", keyIface)
		}

		switch key {
		case "scope":
			err = decoder.Decode(&ss.Scope)
		case "spans":
			err = decoder.Decode(&ss.Spans)
		case "schemaUrl", "schema_url":
			err = decoder.Decode(&ss.SchemaURL)
		default:
			// Skip unknown.
		}

		if err != nil {
			return err
		}
	}
	return nil
}
