// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Scope is the identifying values of the instrumentation scope.
type Scope struct {
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Attrs        []Attr `json:"attributes,omitempty"`
	DroppedAttrs uint32 `json:"droppedAttributesCount,omitempty"`
}
