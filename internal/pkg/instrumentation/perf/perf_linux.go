// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package perf

import (
	"github.com/cilium/ebpf/perf"
)

// Re-export the types from github.com/cilium/ebpf/perf.
type (
	Record        = perf.Record
	Reader        = perf.Reader
	ReaderOptions = perf.ReaderOptions
)

// Re-export the functions from github.com/cilium/ebpf/perf.
// NewReader is a re-export of perf.NewReader.
var NewReader = perf.NewReader

// NewReaderWithOptions is a re-export of perf.NewReaderWithOptions.
var NewReaderWithOptions = perf.NewReaderWithOptions

// ErrClosed is a re-export of perf.ErrClosed.
var ErrClosed = perf.ErrClosed

// ErrFlushed is a re-export of perf.ErrFlushed.
var ErrFlushed = perf.ErrFlushed

// IsUnknownEvent is a re-export of perf.IsUnknownEvent.
var IsUnknownEvent = perf.IsUnknownEvent
