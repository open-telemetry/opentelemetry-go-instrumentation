// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
// TODO: this is not really needed, keeping it for the events map
// once we support a non-reporting probe we can remove this
#include "uprobe.h"

#define MAX_CONCURRENT 100

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct go_iface);
	__uint(max_entries, MAX_CONCURRENT);
} go_context_by_goroutine SEC(".maps");

char __license[] SEC("license") = "Dual MIT/GPL";

const volatile u64 span_context_traceID_offset;
const volatile u64 span_context_spanID_offset;
const volatile u64 span_context_traceFlags_offset;

// This instrumentation attaches uprobe to the following function:
// func SpanContextFromContext(ctx context.Context) SpanContext
SEC("uprobe/SpanContextFromContext")
int uprobe_SpanContextFromContext(struct pt_regs *ctx) {
    struct go_iface go_context = {0};
    get_Go_context(ctx, 1, 0, true, &go_context);
    void *key = (void *)GOROUTINE(ctx);
    struct span_context *sc = span_context_from_go_context(&go_context);
    if (sc == NULL) {
        bpf_printk("entry probe Failed to get span context from go context\n");
    }
    bpf_map_update_elem(&go_context_by_goroutine, &key, &go_context, BPF_ANY);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func SpanContextFromContext(ctx context.Context) SpanContext
SEC("uprobe/SpanContextFromContext_Returns")
int uprobe_SpanContextFromContext_Returns(struct pt_regs *ctx) {
    void *key = (void *)GOROUTINE(ctx);
    struct go_iface *go_context = bpf_map_lookup_elem(&go_context_by_goroutine, &key);
    if (go_context == NULL) {
        bpf_printk("Failed to get go context\n");
        return 0;
    }

    struct span_context *sc = span_context_from_go_context(go_context);
    if (sc == NULL) {
        bpf_printk("Failed to get span context from go context\n");
        goto done;
    }

    void *returned_span_context = (void *)(PT_REGS_SP(ctx) + 8);
    long res = bpf_probe_write_user((void*)(returned_span_context + span_context_traceID_offset), sc->TraceID, sizeof(sc->TraceID));
    if (res != 0) {
        bpf_printk("Failed to write traceID to user space\n");
        goto done;
    }

    res = bpf_probe_write_user((void*)(returned_span_context + span_context_spanID_offset), sc->SpanID, sizeof(sc->SpanID));
    if (res != 0) {
        bpf_printk("Failed to write spanID to user space\n");
        goto done;
    }

    res = bpf_probe_write_user((void*)(returned_span_context + span_context_traceFlags_offset), &sc->TraceFlags, sizeof(sc->TraceFlags));
    if (res != 0) {
        bpf_printk("Failed to write traceFlags to user space\n");
        goto done;
    }

done:
    bpf_map_delete_elem(&go_context_by_goroutine, &key);
    return 0;
}