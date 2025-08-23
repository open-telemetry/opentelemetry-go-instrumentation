// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _GO_CONTEXT_H_
#define _GO_CONTEXT_H_

#include "bpf_helpers.h"
#include "go_types.h"

// This limit is used to define the max length of the context.Context chain
#define MAX_DISTANCE 100
#define MAX_CONCURRENT_SPANS 1000

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct span_context);
    __uint(max_entries, MAX_CONCURRENT_SPANS);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} go_context_to_sc SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct span_context);
    __type(value, void *);
    __uint(max_entries, MAX_CONCURRENT_SPANS);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} tracked_spans_by_sc SEC(".maps");

static __always_inline void *get_parent_go_context(struct go_iface *go_context, void *map) {
    void *data = go_context->data;
    for (int i = 0; i < MAX_DISTANCE; i++) {
        if (data == NULL) {
            break;
        }

        void *found_in_map = bpf_map_lookup_elem(map, &data);
        if (found_in_map != NULL) {
            return data;
        }

        // We assume context.Context implementation contains Parent context.Context member
        // Since the parent is also an interface, we need to read the data part of it
        bpf_probe_read(&data, sizeof(data), data + 8);
    }

    return NULL;
}

static __always_inline struct span_context *get_parent_span_context(struct go_iface *go_context) {
    void *parent_go_ctx = get_parent_go_context(go_context, &go_context_to_sc);
    if (parent_go_ctx == NULL) {
        return NULL;
    }

    struct span_context *parent_sc = bpf_map_lookup_elem(&go_context_to_sc, &parent_go_ctx);
    if (parent_sc == NULL) {
        return NULL;
    }

    return parent_sc;
}

static __always_inline void start_tracking_span(void *contextContext, struct span_context *sc) {
    long err = 0;
    err = bpf_map_update_elem(&go_context_to_sc, &contextContext, sc, BPF_ANY);
    if (err != 0) {
        bpf_printk("Failed to update tracked_spans map: %ld", err);
        return;
    }

    err = bpf_map_update_elem(&tracked_spans_by_sc, sc, &contextContext, BPF_ANY);
    if (err != 0) {
        bpf_printk("Failed to update tracked_spans_by_sc map: %ld", err);
        return;
    }
}

static __always_inline void stop_tracking_span(struct span_context *sc, struct span_context *psc) {
    if (sc == NULL) {
        bpf_printk("stop_tracking_span: sc is null");
        return;
    }

    void *ctx = bpf_map_lookup_elem(&tracked_spans_by_sc, sc);
    if (ctx == NULL) {
        // The span context is not tracked, nothing to do. This can happen for outgoing spans.
        return;
    }

    void *parent_ctx = ((psc == NULL) ? NULL : bpf_map_lookup_elem(&tracked_spans_by_sc, psc));
    if (parent_ctx == NULL) {
        // No parent span, delete the context
        bpf_map_delete_elem(&go_context_to_sc, ctx);
    } else {
        void *ctx_val = 0;
        bpf_probe_read_user(&ctx_val, sizeof(ctx_val), ctx);
        void *parent_ctx_val = 0;
        bpf_probe_read_user(&parent_ctx_val, sizeof(parent_ctx_val), parent_ctx);

        if (ctx_val != parent_ctx_val) {
            // Parent with different context, delete the context
            bpf_map_delete_elem(&go_context_to_sc, ctx);
        } else {
            // Parent with the same context, update the entry to point to the parent span
            bpf_map_update_elem(&go_context_to_sc, ctx, psc, BPF_ANY);
        }
    }

    bpf_map_delete_elem(&tracked_spans_by_sc, sc);
}

//  context_pos:
//      The argument position of the context.Context type pointer
//      In case the context.Context is passed as an argument,
//      this is the argument index of the pointer (starting from 1).
//      In case the context.Context is a member of a struct,
//      this is the argument index of the struct pointer (starting from 1).
//  context_offset:
//      In case the context.Context is a member of a struct,
//      this is the offset of the context.Context member in the struct.
//  passed_as_arg:
//      Indicates whether context.Context is passed as an argument or is a member of a struct
static __always_inline void get_Go_context(void *ctx,
                                           int context_pos,
                                           const volatile u64 context_offset,
                                           bool passed_as_arg,
                                           struct go_iface *contextContext) {
    // Read the argument which is either the context.Context type pointer or pointer to a struct containing the context.Context
    void *ctx_type_or_struct = get_argument(ctx, context_pos);
    if (passed_as_arg) {
        contextContext->type = ctx_type_or_struct;
        contextContext->data = get_argument(ctx, context_pos + 1);
    } else {
        void *context_struct_ptr = (void *)(ctx_type_or_struct + context_offset);
        bpf_probe_read(&contextContext->type, sizeof(contextContext->type), context_struct_ptr);
        bpf_probe_read(&contextContext->data,
                       sizeof(contextContext->data),
                       get_go_interface_instance(context_struct_ptr));
    }
}

#endif
