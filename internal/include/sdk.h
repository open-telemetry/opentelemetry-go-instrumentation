// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef SDK_H
#define SDK_H

#include "trace/span_context.h"

#define MAX_CONCURRENT 50
// TODO: tune this. This is a just a guess, but we should be able to determine
// the maximum size of a span (based on limits) and set this. Ideally, we could
// also look into a tiered allocation strategy so we do not over allocate
// space (i.e. small, medium, large data sizes).
#define MAX_SIZE 2048

// Injected constants
volatile const u64 span_context_trace_id_pos;
volatile const u64 span_context_span_id_pos;
volatile const u64 span_context_trace_flags_pos;

struct otel_span_t {
    struct span_context sc;
    struct span_context psc;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void*);
    __type(value, struct otel_span_t);
    __uint(max_entries, MAX_CONCURRENT);
} active_spans_by_span_ptr SEC(".maps");

// Function declaration
static __always_inline long write_span_context(void *go_sc, struct span_context *sc);

#endif // SDK_H
