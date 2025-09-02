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
    __type(key, void *);
    __type(value, struct otel_span_t);
    __uint(max_entries, MAX_CONCURRENT);
} active_spans_by_span_ptr SEC(".maps");

static __always_inline long write_span_context(void *go_sc, struct span_context *sc) {
    if (go_sc == NULL) {
        bpf_printk("write_span_context: NULL go_sc");
        return -1;
    }

    void *tid = (void *)(go_sc + span_context_trace_id_pos);
    long ret = bpf_probe_write_user(tid, &sc->TraceID, TRACE_ID_SIZE);
    if (ret != 0) {
        bpf_printk("write_span_context: failed to write trace ID: %ld", ret);
        return -2;
    }

    void *sid = (void *)(go_sc + span_context_span_id_pos);
    ret = bpf_probe_write_user(sid, &sc->SpanID, SPAN_ID_SIZE);
    if (ret != 0) {
        bpf_printk("write_span_context: failed to write span ID: %ld", ret);
        return -3;
    }

    void *flags = (void *)(go_sc + span_context_trace_flags_pos);
    ret = bpf_probe_write_user(flags, &sc->TraceFlags, TRACE_FLAGS_SIZE);
    if (ret != 0) {
        bpf_printk("write_span_context: failed to write trace flags: %ld", ret);
        return -4;
    }

    return 0;
}

#endif // SDK_H
