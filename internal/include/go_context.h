// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef _GO_CONTEXT_H_
#define _GO_CONTEXT_H_

#include "bpf_helpers.h"
#include "go_types.h"

// This limit is used to define the max length of the context.Context chain
#define MAX_DISTANCE 100
#define MAX_CONCURRENT_SPANS 1000

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct span_context);
    __uint(max_entries, MAX_CONCURRENT_SPANS);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} tracked_spans SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct span_context);
    __type(value, void *);
    __uint(max_entries, MAX_CONCURRENT_SPANS);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} tracked_spans_by_sc SEC(".maps");

static __always_inline void *get_parent_go_context(void *ctx, void *map) {
    void *data = ctx;
    for (int i = 0; i < MAX_DISTANCE; i++)
    {
        if (data == NULL)
        {
            break;
        }
    
        void *found_in_map = bpf_map_lookup_elem(map, &data);
        if (found_in_map != NULL)
        {
            return data;
        }

        // We assume context.Context implementation contains Parent context.Context member
        // Since the parent is also an interface, we need to read the data part of it
        bpf_probe_read(&data, sizeof(data), data + 8);
    }

    bpf_printk("context %lx not found in context map", ctx);
    return NULL;
}

static __always_inline struct span_context *get_parent_span_context(void *ctx) {
    void *parent_ctx = get_parent_go_context(ctx, &tracked_spans);
    if (parent_ctx == NULL)
    {
        return NULL;
    }

    struct span_context *parent_sc = bpf_map_lookup_elem(&tracked_spans, &parent_ctx);
    if (parent_sc == NULL)
    {
        return NULL;
    }

    return parent_sc;
}

static __always_inline void start_tracking_span(void *contextContext, struct span_context *sc) {
    long err = 0;
    err = bpf_map_update_elem(&tracked_spans, &contextContext, sc, BPF_ANY);
    if (err != 0)
    {
        bpf_printk("Failed to update tracked_spans map: %ld", err);
        return;
    }

    err = bpf_map_update_elem(&tracked_spans_by_sc, sc, &contextContext, BPF_ANY);
    if (err != 0)
    {
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
    if (ctx == NULL)
    {
        bpf_printk("stop_tracking_span: can't find span context");
        return;
    }

    void *parent_ctx = ((psc == NULL) ? NULL : bpf_map_lookup_elem(&tracked_spans_by_sc, psc));
    if (parent_ctx == NULL)
    {
        // No parent span, delete the context
        bpf_map_delete_elem(&tracked_spans, ctx);
    } else 
    {
        void *ctx_val = 0;
        bpf_probe_read_user(&ctx_val, sizeof(ctx_val), ctx);
        void *parent_ctx_val = 0;
        bpf_probe_read_user(&parent_ctx_val, sizeof(parent_ctx_val), parent_ctx);

        if (ctx_val != parent_ctx_val)
        {
            // Parent with different context, delete the context
            bpf_map_delete_elem(&tracked_spans, ctx);
        } else {
            // Parent with the same context, update the entry to point to the parent span
            bpf_map_update_elem(&tracked_spans, ctx, psc, BPF_ANY);
        }
    }

    bpf_map_delete_elem(&tracked_spans_by_sc, sc);
}

// Extract the go context.Context data pointer from the function arguments
// context_pos:
    // The argument position of the context.Context data pointer
    // In case the context.Context is passed as an argument,
    // this is the argument index of the pointer (starting from 1).
    // In case the context.Context is a member of a struct,
    // this is the argument index of the struct pointer (starting from 1). 
// context_offset:
    // In case the context.Context is a member of a struct,
    // this is the offset of the context.Context member in the struct.
// passed_as_arg:
    // Indicates whether context.Context is passed as an argument or is a member of a struct
static __always_inline void *get_Go_context(void *ctx, int context_pos, const volatile u64 context_offset, bool passed_as_arg) {
    void *arg = get_argument(ctx, context_pos);
    if (passed_as_arg) {
        return arg;
    }
    void *ctx_addr = get_go_interface_instance((void*)(arg + context_offset));
    void *ctx_val = 0;
    bpf_probe_read_user(&ctx_val, sizeof(ctx_val), ctx_addr);
    return ctx_val;
}

#endif