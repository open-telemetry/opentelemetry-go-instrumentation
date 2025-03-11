// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package perf

import (
	"github.com/cilium/ebpf/perf"
)

// Re-export the types from github.com/cilium/ebpf/perf
type Record = perf.Record
type Reader = perf.Reader
type ReaderOptions = perf.ReaderOptions

// Re-export the functions from github.com/cilium/ebpf/perf
var NewReader = perf.NewReader
var NewReaderWithOptions = perf.NewReaderWithOptions
var ErrClosed = perf.ErrClosed
var ErrFlushed = perf.ErrFlushed
var IsUnknownEvent = perf.IsUnknownEvent
