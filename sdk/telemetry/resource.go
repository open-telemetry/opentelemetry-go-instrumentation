// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Resource information.
type Resource struct {
	// Attrs are the set of attributes that describe the resource. Attribute
	// keys MUST be unique (it is not allowed to have more than one attribute
	// with the same key).
	Attrs []Attr `json:"attributes"`
	// DroppedAttrs is the number of dropped attributes. If the value
	// is 0, then no attributes were dropped.
	DroppedAttrs uint32 `json:"droppedAttributesCount,omitempty"`
}
