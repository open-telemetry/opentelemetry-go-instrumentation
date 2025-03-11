// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

// Package perf provides stub implementation for non-Linux platforms.
// These stubs allow the code to compile but will not work at runtime.
package perf

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cilium/ebpf"
)

// ErrClosed is returned when interacting with a closed Reader.
var ErrClosed = os.ErrClosed

// ErrFlushed is returned when the Reader has been flushed.
var ErrFlushed = errors.New("perf reader flushed")

// Record contains data from a perf event.
type Record struct {
	// The CPU this record was generated on.
	CPU int

	// The data submitted via bpf_perf_event_output.
	RawSample []byte

	// The number of samples which could not be output, since
	// the ring buffer was full.
	LostSamples uint64

	// The minimum number of bytes remaining in the per-CPU buffer after this Record has been read.
	Remaining int
}

// Reader allows reading from a perf event array.
type Reader struct{}

// ReaderOptions control the behavior of the Reader.
type ReaderOptions struct {
	// The number of events required in any per CPU buffer before
	// Read will process data. This is mutually exclusive with Watermark.
	WakeupEvents int
	// The number of written bytes required in any per CPU buffer before
	// Read will process data. Must be smaller than PerCPUBuffer.
	Watermark int
	// This perf ring buffer is overwritable, once full the oldest event will be
	// overwritten by newest.
	Overwritable bool
}

// NewReader creates a new reader with default options.
func NewReader(array *ebpf.Map, perCPUBuffer int) (*Reader, error) {
	return nil, errors.New("perf.Reader not supported on non-Linux platforms")
}

// NewReaderWithOptions creates a new reader with the given options.
func NewReaderWithOptions(array *ebpf.Map, perCPUBuffer int, opts ReaderOptions) (*Reader, error) {
	return nil, errors.New("perf.Reader not supported on non-Linux platforms")
}

// Close frees resources used by the reader.
func (r *Reader) Close() error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// Read the next record from the perf ring buffer.
func (r *Reader) Read() (Record, error) {
	return Record{}, fmt.Errorf("perf reader: %w", ErrClosed)
}

// ReadInto is like Read but allows reusing the Record.
func (r *Reader) ReadInto(rec *Record) error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// SetDeadline controls how long Read and ReadInto will block.
func (r *Reader) SetDeadline(t time.Time) {
	// No-op on non-Linux
}

// Pause stops all notifications from this Reader.
func (r *Reader) Pause() error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// Resume allows this perf reader to emit notifications.
func (r *Reader) Resume() error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// FlushAndClose flushes all pending events and closes the reader.
func (r *Reader) FlushAndClose() error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// Flush unblocks Read/ReadInto and returns pending samples.
func (r *Reader) Flush() error {
	return fmt.Errorf("perf reader: %w", ErrClosed)
}

// BufferSize returns the size in bytes of each per-CPU buffer.
func (r *Reader) BufferSize() int {
	return 0
}

// IsUnknownEvent returns true if the error occurred because an
// unknown event was submitted to the perf event ring.
func IsUnknownEvent(err error) bool {
	return false
}
