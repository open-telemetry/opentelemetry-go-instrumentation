#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100


struct grpc_request_t {
    char method[MAX_SIZE];
    char target[MAX_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// This instrumentation attaches uprobe to the following function;
// func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error
SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke(struct pt_regs *ctx) {
    // positions
    u64 clientconn_pos = 1;
    u64 context_pos = 2;
    u64 method_ptr_pos = 4;
    u64 method_len_pos = 5;
    u64 clientconn_target_ptr_pos = 3;
    u64 clientconn_target_len_pos = 4;

    struct grpc_request_t grpcReq = {};

    // Read Method
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(ctx->rsp+(method_ptr_pos*8)));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(ctx->rsp+(method_len_pos*8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Read ClientConn.Target
    void* clientconn_ptr = 0;
    bpf_probe_read(&clientconn_ptr, sizeof(clientconn_ptr), (void *)(ctx->rsp+(clientconn_pos*8)));
    void* target_ptr = 0;
    bpf_probe_read(&target_ptr, sizeof(target_ptr), (void *)(clientconn_ptr+(clientconn_target_ptr_pos*8)));
    u64 target_len = 0;
    bpf_probe_read(&target_len, sizeof(target_len), (void *)(clientconn_ptr+(clientconn_target_len_pos*8)));
    u64 target_size = sizeof(grpcReq.target);
    target_size = target_size < target_len ? target_size : target_len;
    bpf_probe_read(&grpcReq.target, target_size, target_ptr);

    // Write event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    return 0;
}

SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke_ByRegisters(struct pt_regs *ctx) {
    // positions
    u64 clientconn_target_ptr_pos = 3;
    u64 clientconn_target_len_pos = 4;

    struct grpc_request_t grpcReq = {};

    // Read Method
    void* method_ptr = (void *)(ctx->rdi);
    u64 method_len = (u64)(ctx->rsi);
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Read ClientConn.Target
    void* clientconn_ptr = (void *)(ctx->rax);
    void* target_ptr = 0;
    bpf_probe_read(&target_ptr, sizeof(target_ptr), (void *)(clientconn_ptr+(clientconn_target_ptr_pos*8)));
    u64 target_len = 0;
    bpf_probe_read(&target_len, sizeof(target_len), (void *)(clientconn_ptr+(clientconn_target_len_pos*8)));
    u64 target_size = sizeof(grpcReq.target);
    target_size = target_size < target_len ? target_size : target_len;
    bpf_probe_read(&grpcReq.target, target_size, target_ptr);

    // Write event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    return 0;
}