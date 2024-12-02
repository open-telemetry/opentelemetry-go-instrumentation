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

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
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
    u64 rctx_ptr;
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
} chi_uprobe_storage_map SEC(".maps");

// Injected in init
volatile const u64 pctx_ptr_pos;
volatile const u64 rp_str_pos;
volatile const u64 method_str_pos;

// This instrumentation attaches uprobe to the following function:
// func (mx *Mux) routeHTTP(w http.ResponseWriter, r *http.Request)
SEC("uprobe/chi_node_FindRoute")
int uprobe_chi_node_FindRoute(struct pt_regs *ctx) {
    u64 rctx_pos = 2;
    void *rctx_ptr = get_argument(ctx, rctx_pos);

    struct go_iface go_context = {0};
    get_Go_context(ctx, rctx_pos, pctx_ptr_pos, false, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);

    u32 map_id = 0;
    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&chi_uprobe_storage_map, &map_id);
    if (uprobe_data == NULL)
    {
        bpf_printk("uprobe/chi_node_FindRoute: http_server_span is NULL");
        return 0;
    }

    __builtin_memset(uprobe_data, 0, sizeof(struct uprobe_data_t));

    // Save Request
    uprobe_data->rctx_ptr = (u64)rctx_ptr;

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

    // get path from 3rd arg
    void *path_str_ptr = get_argument(ctx, 4);
    u64 path_str_len = (u64)get_argument(ctx, 5);
    const u64 path_str_size = sizeof(http_request->path) < path_str_len ? sizeof(http_request->path) : path_str_len;
    bpf_probe_read(http_request->path, path_str_size, path_str_ptr);

    bpf_map_update_elem(&http_events, &key, uprobe_data, 0);
    start_tracking_span(go_context.data, &http_request->sc);
    return 0;
}

SEC("uprobe/chi_node_FindRoute")
int uprobe_chi_node_FindRoute_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, pctx_ptr_pos, false, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);

    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&http_events, &key);
    if (uprobe_data == NULL) {
        bpf_printk("uprobe/chi_node_FindRoute: entry_state is NULL");
        return 0;
    }

    struct http_request_t *http_request = &uprobe_data->req;
    http_request->end_time = end_time;

    void *rctx_ptr = (void *)uprobe_data->rctx_ptr;

    // Get method from rctx
    if (!get_go_string_from_user_ptr((void *)(rctx_ptr + method_str_pos), http_request->method, sizeof(http_request->method))) {
        bpf_printk("failed to get method from rctx");
    }

    // get rctx.routePattern
    if (!get_go_string_from_user_ptr((void *)(rctx_ptr + rp_str_pos), http_request->path_pattern, sizeof(http_request->path_pattern))) {
        bpf_printk("failed to get path_pattern from rctx");
    }

    bpf_map_update_elem(&http_events, &key, uprobe_data, 0);

    output_span_event(ctx, http_request, sizeof(*http_request), &http_request->sc);

    stop_tracking_span(&http_request->sc, &http_request->psc);
    bpf_map_delete_elem(&http_events, &key);
    return 0;
}
