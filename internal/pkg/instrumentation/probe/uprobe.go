// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package probe provides instrumentation probe types and definitions.
package probe

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Uprobe is an eBPF program that is attached in the entry point and/or the return of a function.
type Uprobe struct {
	// Sym is the symbol name of the function to attach the eBPF program to.
	Sym string
	// Optional is a boolean flag informing if the Uprobe is optional. If the
	// Uprobe is optional and fails to attach, the error is logged and
	// processing continues.
	Optional bool
	// EntryProbe is the name of the eBPF program to attach to the entry of the
	// function specified by Sym. If EntryProbe is empty, no eBPF program will be attached to the entry of the function.
	EntryProbe string
	// ReturnProbe is the name of the eBPF program to attach to the return of the
	// function specified by Sym. If ReturnProbe is empty, no eBPF program will be attached to the return of the function.
	ReturnProbe string
	DependsOn   []string
}

func (u *Uprobe) load(exec *link.Executable, target *process.TargetDetails, c *ebpf.Collection) ([]link.Link, error) {
	offset, err := target.GetFunctionOffset(u.Sym)
	if err != nil {
		return nil, err
	}

	var links []link.Link

	if u.EntryProbe != "" {
		entryProg, ok := c.Programs[u.EntryProbe]
		if !ok {
			return nil, fmt.Errorf("entry probe %s not found", u.EntryProbe)
		}
		opts := &link.UprobeOptions{Address: offset, PID: target.PID}
		l, err := exec.Uprobe("", entryProg, opts)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}

	if u.ReturnProbe != "" {
		retProg, ok := c.Programs[u.ReturnProbe]
		if !ok {
			return nil, fmt.Errorf("return probe %s not found", u.ReturnProbe)
		}
		retOffsets, err := target.GetFunctionReturns(u.Sym)
		if err != nil {
			return nil, err
		}

		for _, ret := range retOffsets {
			opts := &link.UprobeOptions{Address: ret, PID: target.PID}
			l, err := exec.Uprobe("", retProg, opts)
			if err != nil {
				return nil, err
			}
			links = append(links, l)
		}
	}

	return links, nil
}
