#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100

struct grpc_request_t {
    char method[MAX_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo) {
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_ByRegisters(struct pt_regs *ctx) {
    // Positions of method inside transport.Stream struct
    u64 stream_method_ptr_pos = 9;
    u64 stream_method_len_pos = 10;

    struct grpc_request_t grpcReq = {};
    void* stream_ptr = (void *)(ctx->rdi);
    void* method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr+(stream_method_ptr_pos*8)));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr+(stream_method_len_pos*8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Write event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    return 0;
}