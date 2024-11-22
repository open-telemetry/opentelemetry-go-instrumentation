// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

// NewTargetSpanProducingProbe returns a fully instantiated TargetSpanProducingProbe.
func NewTargetSpanProducingProbe[BPFObj any, BPFEvent any]() *TargetSpanProducingProbe[BPFObj, BPFEvent] {
	return &TargetSpanProducingProbe[BPFObj, BPFEvent]{
		TracingConfig:             &TracingConfig{},
		TargetEventProducingProbe: NewTargetEventProducingProbe[BPFObj, BPFEvent](),
	}
}

// NewTargetTraceProducingProbe returns a fully instantiated TargetTraceProducingProbe.
func NewTargetTraceProducingProbe[BPFObj any, BPFEvent any]() *TargetTraceProducingProbe[BPFObj, BPFEvent] {
	return &TargetTraceProducingProbe[BPFObj, BPFEvent]{
		TracingConfig:             &TracingConfig{},
		TargetEventProducingProbe: NewTargetEventProducingProbe[BPFObj, BPFEvent](),
	}
}

// NewTargetEventProducingProbe returns a fully instantiated TargetEventProducingProbe.
func NewTargetEventProducingProbe[BPFObj any, BPFEvent any]() *TargetEventProducingProbe[BPFObj, BPFEvent] {
	return &TargetEventProducingProbe[BPFObj, BPFEvent]{
		TargetExecutableProbe: NewTargetExecutableProbe[BPFObj](),
	}
}

// NewTargetExectuableProbe returns a fully instantiated TargetExecutableProbe.
func NewTargetExecutableProbe[BPFObj any]() *TargetExecutableProbe[BPFObj] {
	return &TargetExecutableProbe[BPFObj]{
		TargetExecutableConfig: &TargetExecutableConfig{},
		BasicProbe:             NewBasicProbe(),
	}
}

// NewBasicProbe returns a fully instantiated BasicProbe.
func NewBasicProbe() *BasicProbe {
	return &BasicProbe{}
}
