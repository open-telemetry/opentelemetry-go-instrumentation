#include "arguments.h"
#include "span_context.h"
#include "go_context.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100
#define MAX_CONCURRENT 50

struct http_request_t {
    u64 start_time;
    u64 end_time;
    char method[MAX_SIZE];
    char path[MAX_SIZE];
    struct span_context sc;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct http_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} context_to_http_events SEC(".maps");

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
SEC("uprobe/ServerMux_ServeHTTP")
int uprobe_ServerMux_ServeHTTP(struct pt_regs *ctx) {
    u64 request_pos = 4;
    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_boot_ns();

    // Get request struct
    void* req_ptr = get_argument(ctx, request_pos);

    // Get method from request
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(req_ptr+method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(req_ptr+(method_ptr_pos+8)));
    u64 method_size = sizeof(httpReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&httpReq.method, method_size, method_ptr);

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr+url_ptr_pos));
    void* path_ptr = 0;
    bpf_probe_read(&path_ptr, sizeof(path_ptr), (void *)(url_ptr+path_ptr_pos));
    u64 path_len = 0;
    bpf_probe_read(&path_len, sizeof(path_len), (void *)(url_ptr+(path_ptr_pos+8)));
    u64 path_size = sizeof(httpReq.path);
    path_size = path_size < path_len ? path_size : path_len;
    bpf_probe_read(&httpReq.path, path_size, path_ptr);

    // Get Request.ctx
    void *ctx_iface = 0;
    bpf_probe_read(&ctx_iface, sizeof(ctx_iface), (void *)(req_ptr+ctx_ptr_pos+8));

    // Write event
    httpReq.sc = generate_span_context();
    bpf_map_update_elem(&context_to_http_events, &ctx_iface, &httpReq, 0);
    long res = bpf_map_update_elem(&spans_in_progress, &ctx_iface, &httpReq.sc, 0);
    return 0;
}

SEC("uprobe/ServerMux_ServeHTTP")
int uprobe_ServerMux_ServeHTTP_Returns(struct pt_regs *ctx) {
    u64 request_pos = 4;
    void* req_ptr = get_argument(ctx, request_pos);
    void *ctx_iface = 0;
    bpf_probe_read(&ctx_iface, sizeof(ctx_iface), (void *)(req_ptr+ctx_ptr_pos+8));

    void* httpReq_ptr = bpf_map_lookup_elem(&context_to_http_events, &ctx_iface);
    struct http_request_t httpReq = {};
    bpf_probe_read(&httpReq, sizeof(httpReq), httpReq_ptr);
    httpReq.end_time = bpf_ktime_get_boot_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &httpReq, sizeof(httpReq));
    bpf_map_delete_elem(&context_to_http_events, &ctx_iface);
    bpf_map_delete_elem(&spans_in_progress, &ctx_iface);
    return 0;
}