// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// Const is an constant that needs to be injected into an eBPF program.
type Const interface {
	// InjectOption returns the inject.Option to run for the Const when running
	// inject.Constants.
	InjectOption(td *process.TargetDetails) (inject.Option, error)
}

// StructFieldConst is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package.
type StructFieldConst struct {
	Key string
	Val structfield.ID
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known. If it is not, an error is
// returned.
func (c StructFieldConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	ver, ok := td.Libraries[c.Val.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", c.Val.ModPath)
	}
	return inject.WithOffset(c.Key, c.Val, ver), nil
}

// StructFieldConstMinVersion is a [Const] for a struct field offset. These struct field
// ID needs to be known offsets in the [inject] package. The offset is only
// injected if the module version is greater than or equal to the MinVersion.
type StructFieldConstMinVersion struct {
	StructField StructFieldConst
	MinVersion  *version.Version
}

// InjectOption returns the appropriately configured [inject.WithOffset] if the
// version of the struct field module is known and is greater than or equal to
// the MinVersion. If the module version is not known, an error is returned.
// If the module version is known but is less than the MinVersion, no offset is
// injected.
func (c StructFieldConstMinVersion) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	sf := c.StructField
	ver, ok := td.Libraries[sf.Val.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", sf.Val.ModPath)
	}

	if !ver.GreaterThanOrEqual(c.MinVersion) {
		return nil, nil
	}

	return inject.WithOffset(sf.Key, sf.Val, ver), nil
}

// AllocationConst is a [Const] for all the allocation details that need to be
// injected into an eBPF program.
type AllocationConst struct{}

// InjectOption returns the appropriately configured
// [inject.WithAllocationDetails] if the [process.AllocationDetails] within td
// are not nil. An error is returned if [process.AllocationDetails] is nil.
func (c AllocationConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	if td.AllocationDetails == nil {
		return nil, errors.New("no allocation details")
	}
	return inject.WithAllocationDetails(*td.AllocationDetails), nil
}

// RegistersABIConst is a [Const] for the boolean flag informing an eBPF
// program if the Go space has registered ABI.
type RegistersABIConst struct{}

// InjectOption returns the appropriately configured [inject.WithRegistersABI].
func (c RegistersABIConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	return inject.WithRegistersABI(td.IsRegistersABI()), nil
}

// KeyValConst is a [Const] for a generic key-value pair.
//
// This should not be used as a replacement for any of the other provided
// [Const] implementations. Those implementations may have added significance
// and should be used instead where applicable.
type KeyValConst struct {
	Key string
	Val interface{}
}

// InjectOption returns the appropriately configured [inject.WithKeyValue].
func (c KeyValConst) InjectOption(*process.TargetDetails) (inject.Option, error) {
	return inject.WithKeyValue(c.Key, c.Val), nil
}
