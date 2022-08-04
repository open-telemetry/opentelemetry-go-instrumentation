// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
/* Copyright Authors of OpenTelemetry */

#include "arguments.h"
#include "goroutines.h"

char __license[] SEC("license") = "Dual BSD/GPL";

#define MAX_SIZE 100
#define MAX_CONCURRENT 50

struct grpc_request_t {
    s64 goroutine;
    u64 start_time;
    u64 end_time;
    char method[MAX_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, s64);
	__type(value, struct grpc_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} goid_to_grpc_events SEC(".maps");

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
    grpcReq.start_time = bpf_ktime_get_ns();

    void* stream_ptr = get_argument(ctx, stream_pos);
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr+stream_method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr+(stream_method_ptr_pos+8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Record goroutine
    grpcReq.goroutine = get_current_goroutine();

    // Write event
    bpf_map_update_elem(&goid_to_grpc_events, &grpcReq.goroutine, &grpcReq, 0);

    return 0;
}

SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_ByRegisters(struct pt_regs *ctx) {
    struct grpc_request_t grpcReq = {};
    grpcReq.start_time = bpf_ktime_get_ns();
    void* stream_ptr = (void *)(ctx->rdi);
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
    bpf_map_update_elem(&goid_to_grpc_events, &goid, &grpcReq, 0);
    return 0;
}

SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_Returns(struct pt_regs *ctx) {
    s64 goid = get_current_goroutine();
    void* grpcReq_ptr = bpf_map_lookup_elem(&goid_to_grpc_events, &goid);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);
    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&goid_to_grpc_events, &goid);
    return 0;
}