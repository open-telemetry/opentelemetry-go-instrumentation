// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"log/slog"
	"testing"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
)

func TestClientHeadersVersionSplit(t *testing.T) {
	p := New(slog.Default(), "").(*probe.SpanProducer[bpfObjects, event])

	const (
		oldSym = "google.golang.org/grpc/internal/transport.(*loopyWriter).headerHandler"
		newSym = "google.golang.org/grpc/internal/transport.(*loopyWriter).clientHeaderHandler"
	)
	uprobes := make(map[string]*probe.Uprobe)
	for _, uprobe := range p.Uprobes {
		if uprobe.Sym == oldSym || uprobe.Sym == newSym {
			uprobes[uprobe.Sym] = uprobe
		}
	}

	tests := []struct {
		version string
		old     bool
		new     bool
	}{
		{version: "1.82.0", old: true, new: false},
		{version: "1.82.1", old: false, new: true},
	}
	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			version := semver.MustParse(test.version)
			for symbol, want := range map[string]bool{oldSym: test.old, newSym: test.new} {
				uprobe, ok := uprobes[symbol]
				if !ok {
					t.Fatalf("uprobe %q not found", symbol)
				}
				if len(uprobe.PackageConstraints) != 1 {
					t.Fatalf("uprobe %q has %d constraints, want 1", symbol, len(uprobe.PackageConstraints))
				}
				if got := uprobe.PackageConstraints[0].Constraints.Check(version); got != want {
					t.Errorf("uprobe %q match = %t, want %t", symbol, got, want)
				}
			}
		})
	}

	var oldFields, newFields int
	for _, cnst := range p.Consts {
		switch value := cnst.(type) {
		case probe.StructFieldConstMaxVersion:
			if value.StructField.ID.Struct == "headerFrame" {
				oldFields++
				if !value.MaxVersion.Equal(clientHeadersVersion) {
					t.Errorf("headerFrame max version = %s, want %s", value.MaxVersion, clientHeadersVersion)
				}
			}
		case probe.StructFieldConstMinVersion:
			if value.StructField.ID.Struct == "clientHeaders" {
				newFields++
				if !value.MinVersion.Equal(clientHeadersVersion) {
					t.Errorf("clientHeaders min version = %s, want %s", value.MinVersion, clientHeadersVersion)
				}
			}
		}
	}
	if oldFields != 2 {
		t.Errorf("headerFrame field constants = %d, want 2", oldFields)
	}
	if newFields != 2 {
		t.Errorf("clientHeaders field constants = %d, want 2", newFields)
	}

	var manifestOldFields, manifestNewFields int
	for _, field := range p.Manifest().StructFields {
		switch field.Struct {
		case "headerFrame":
			manifestOldFields++
		case "clientHeaders":
			manifestNewFields++
		}
	}
	if manifestOldFields != 2 {
		t.Errorf("manifest headerFrame fields = %d, want 2", manifestOldFields)
	}
	if manifestNewFields != 2 {
		t.Errorf("manifest clientHeaders fields = %d, want 2", manifestNewFields)
	}
}
