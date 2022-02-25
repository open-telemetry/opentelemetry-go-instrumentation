#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, s64);
	__uint(max_entries, MAX_OS_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

struct grpc_request_t {
    s64 goroutine;
    char method[MAX_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 stream_method_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo) {
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream(struct pt_regs *ctx) {
    u64 stream_pos = 4;

    struct grpc_request_t grpcReq = {};
    void* stream_ptr = 0;
    bpf_probe_read(&stream_ptr, sizeof(stream_ptr), (void *)(ctx->rsp+(stream_pos*8)));
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr+stream_method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr+(stream_method_ptr_pos+8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Record goroutine
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    s64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    grpcReq.goroutine = goid;

    // Write event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    return 0;
}

SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_ByRegisters(struct pt_regs *ctx) {
    struct grpc_request_t grpcReq = {};
    void* stream_ptr = (void *)(ctx->rdi);
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr+stream_method_ptr_pos-8));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr+(stream_method_ptr_pos)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Record goroutine
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    s64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    grpcReq.goroutine = goid;

    // Write event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    return 0;
}