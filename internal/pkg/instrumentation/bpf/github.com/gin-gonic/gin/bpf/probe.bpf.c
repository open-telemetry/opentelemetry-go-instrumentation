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
#include "span_context.h"
#include "go_context.h"
#include "uprobe.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define PATH_MAX_LEN 100
#define METHOD_MAX_LEN 7
#define MAX_CONCURRENT 50

struct http_request_t {
    BASE_SPAN_PROPERTIES
    char method[METHOD_MAX_LEN];
    char path[PATH_MAX_LEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct http_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} http_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 ctx_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (engine *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request)
SEC("uprobe/GinEngine_ServeHTTP")
int uprobe_GinEngine_ServeHTTP(struct pt_regs *ctx) {
    u64 request_pos = 4;
    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_ns();

    // Get request struct
    void *req_ptr = get_argument(ctx, request_pos);
    void *req_ctx_ptr = get_Go_context(ctx, 4, ctx_ptr_pos, false);

    // Get method from request
    if (!get_go_string_from_user_ptr((void *)(req_ptr + method_ptr_pos), httpReq.method, sizeof(httpReq.method))) {
        bpf_printk("failed to get method from request");
        return 0;
    }

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr + url_ptr_pos));
    if (!get_go_string_from_user_ptr((void *)(url_ptr + path_ptr_pos), httpReq.path, sizeof(httpReq.path))) {
        bpf_printk("failed to get path from Request.URL");
        return 0;
    }

    // Get key
    void *key = get_consistent_key(ctx, req_ctx_ptr);

    // Write event
    httpReq.sc = generate_span_context();
    bpf_map_update_elem(&http_events, &key, &httpReq, 0);
    start_tracking_span(req_ctx_ptr, &httpReq.sc);
    return 0;
}

UPROBE_RETURN(GinEngine_ServeHTTP, struct http_request_t, http_events, events, 4, ctx_ptr_pos, false)