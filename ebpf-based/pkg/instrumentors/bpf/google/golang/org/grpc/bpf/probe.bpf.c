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
    char target[MAX_SIZE];
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
volatile const u64 clientconn_target_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error
SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke(struct pt_regs *ctx) {
    // positions
    u64 clientconn_pos = 1;
    u64 context_pos = 2;
    u64 method_ptr_pos = 4;
    u64 method_len_pos = 5;

    struct grpc_request_t grpcReq = {};
    grpcReq.start_time = bpf_ktime_get_ns();

    // Read Method
    void* method_ptr = get_argument(ctx, method_ptr_pos);
    u64 method_len = (u64) get_argument(ctx, method_len_pos);
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Read ClientConn.Target
    void* clientconn_ptr = get_argument(ctx, clientconn_pos);
    void* target_ptr = 0;
    bpf_probe_read(&target_ptr, sizeof(target_ptr), (void *)(clientconn_ptr+(clientconn_target_ptr_pos)));
    u64 target_len = 0;
    bpf_probe_read(&target_len, sizeof(target_len), (void *)(clientconn_ptr+(clientconn_target_ptr_pos+8)));
    u64 target_size = sizeof(grpcReq.target);
    target_size = target_size < target_len ? target_size : target_len;
    bpf_probe_read(&grpcReq.target, target_size, target_ptr);

    // Record goroutine
    grpcReq.goroutine = get_current_goroutine();

    // Write event
    bpf_map_update_elem(&goid_to_grpc_events, &grpcReq.goroutine, &grpcReq, 0);

    return 0;
}

SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke_Returns(struct pt_regs *ctx) {
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    s64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);

    void* grpcReq_ptr = bpf_map_lookup_elem(&goid_to_grpc_events, &goid);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);
    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&goid_to_grpc_events, &goid);

    return 0;
}