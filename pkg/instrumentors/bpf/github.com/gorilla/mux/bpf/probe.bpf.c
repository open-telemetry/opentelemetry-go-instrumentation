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

char __license[] SEC("license") = "Dual MIT/GPL";

#define PATH_MAX_LEN 100
#define METHOD_MAX_LEN 7
#define MAX_CONCURRENT 50

struct http_request_t {
    u64 start_time;
    u64 end_time;
    char method[METHOD_MAX_LEN];
    char path[PATH_MAX_LEN];
    struct span_context sc;
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
// func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request)
SEC("uprobe/GorillaMux_ServeHTTP")
int uprobe_GorillaMux_ServeHTTP(struct pt_regs *ctx) {
    u64 request_pos = 4;
    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_ns();

    // Get request struct
    void *req_ptr = get_argument(ctx, request_pos);

    // Get method from request
    void *method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(req_ptr + method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(req_ptr + (method_ptr_pos + 8)));
    u64 method_size = sizeof(httpReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&httpReq.method, method_size, method_ptr);

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr + url_ptr_pos));
    void *path_ptr = 0;
    bpf_probe_read(&path_ptr, sizeof(path_ptr), (void *)(url_ptr + path_ptr_pos));
    u64 path_len = 0;
    bpf_probe_read(&path_len, sizeof(path_len), (void *)(url_ptr + (path_ptr_pos + 8)));
    u64 path_size = sizeof(httpReq.path);
    path_size = path_size < path_len ? path_size : path_len;
    bpf_probe_read(&httpReq.path, path_size, path_ptr);

    // Get key
    void *req_ctx_ptr = 0;
    bpf_probe_read(&req_ctx_ptr, sizeof(req_ctx_ptr), (void *)(req_ptr + ctx_ptr_pos));
    void *key = get_consistent_key(ctx, (void *)(req_ptr + ctx_ptr_pos));

    // Write event
    httpReq.sc = generate_span_context();
    bpf_map_update_elem(&http_events, &key, &httpReq, 0);
    track_running_span(req_ctx_ptr, &httpReq.sc);
    return 0;
}

SEC("uprobe/GorillaMux_ServeHTTP")
int uprobe_GorillaMux_ServeHTTP_Returns(struct pt_regs *ctx) {
    u64 request_pos = 4;
    void* req_ptr = get_argument(ctx, request_pos);

    // Get key
    void *key = get_consistent_key(ctx, (void *)(req_ptr + ctx_ptr_pos));

    void *httpReq_ptr = bpf_map_lookup_elem(&http_events, &key);
    struct http_request_t httpReq = {};
    bpf_probe_read(&httpReq, sizeof(httpReq), httpReq_ptr);
    httpReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &httpReq, sizeof(httpReq));
    bpf_map_delete_elem(&http_events, &key);
    stop_tracking_span(&httpReq.sc);
    return 0;
}