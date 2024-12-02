// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "uprobe.h"
#include "trace/span_output.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define PATH_MAX_LEN 128
#define METHOD_MAX_LEN 8
#define MAX_CONCURRENT 50

struct http_request_t {
    BASE_SPAN_PROPERTIES
    char method[METHOD_MAX_LEN];
    char path[PATH_MAX_LEN];
    char path_pattern[PATH_MAX_LEN];
};

struct uprobe_data_t
{
    struct http_request_t req;
    u64 gin_ctx_ptr;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct uprobe_data_t);
    __uint(max_entries, MAX_CONCURRENT);
} http_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct uprobe_data_t));
    __uint(max_entries, 1);
} gin_uprobe_storage_map SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 ctx_ptr_pos;
volatile const u64 req_ptr_pos;
volatile const u64 fullpath_str_pos;

// This instrumentation attaches uprobe to the following function:
// func (engine *Engine) handleHTTPRequest(c *Context)
SEC("uprobe/GinEngine_handleHTTPRequest")
int uprobe_GinEngine_handleHTTPRequest(struct pt_regs *ctx) {
    u64 gin_ctx_pos = 2;
    void *gin_ctx_ptr = get_argument(ctx, gin_ctx_pos);

    struct go_iface go_context = {0};
    get_Go_context(ctx, gin_ctx_pos, ctx_ptr_pos, false, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);

    u32 map_id = 0;
    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&gin_uprobe_storage_map, &map_id);
    if (uprobe_data == NULL)
    {
        bpf_printk("uprobe/GinEngine_handleHTTPRequest: http_server_span is NULL");
        return 0;
    }

    __builtin_memset(uprobe_data, 0, sizeof(struct uprobe_data_t));

    // Save gin ctx
    uprobe_data->gin_ctx_ptr = (u64)gin_ctx_ptr;

    struct http_request_t *http_request = &uprobe_data->req;
    http_request->start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &http_request->psc,
        .sc = &http_request->sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    bpf_map_update_elem(&http_events, &key, uprobe_data, 0);
    start_tracking_span(go_context.data, &http_request->sc);
    return 0;
}

SEC("uprobe/GinEngine_handleHTTPRequest")
int uprobe_GinEngine_handleHTTPRequest_Returns(struct pt_regs *ctx) {
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, ctx_ptr_pos, false, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);

    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&http_events, &key);
    if (uprobe_data == NULL) {
        bpf_printk("uprobe/GinEngine_handleHTTPRequest: entry_state is NULL");
        return 0;
    }

    struct http_request_t *http_request = &uprobe_data->req;

    http_request->end_time = bpf_ktime_get_ns();

    void *gin_ctx_ptr = (void *)uprobe_data->gin_ctx_ptr;

    // Get method from request
    void *req_ptr = 0;
    bpf_probe_read(&req_ptr, sizeof(req_ptr), (void *)(gin_ctx_ptr + req_ptr_pos));
    if (!get_go_string_from_user_ptr((void *)(req_ptr + method_ptr_pos), http_request->method, sizeof(http_request->method))) {
        bpf_printk("failed to get method from request");
    }

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr + url_ptr_pos));
    if (!get_go_string_from_user_ptr((void *)(url_ptr + path_ptr_pos), http_request->path, sizeof(http_request->path))) {
        bpf_printk("failed to get path from Request.URL");
    }

    // get pathPattern
    if (!get_go_string_from_user_ptr((void *)(gin_ctx_ptr + fullpath_str_pos), http_request->path_pattern, sizeof(http_request->path_pattern))) {
        bpf_printk("failed to get path_pattern from gin context");
    }

    bpf_map_update_elem(&http_events, &key, http_request, 0);

    output_span_event(ctx, http_request, sizeof(*http_request), &http_request->sc);

    stop_tracking_span(&http_request->sc, &http_request->psc);
    bpf_map_delete_elem(&http_events, &key);
    return 0;
}
