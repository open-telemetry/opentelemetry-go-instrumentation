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
#include "go_types.h"
#include "span_context.h"
#include "go_context.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 50
#define MAX_CONCURRENT 50
#define MAX_HEADERS_BUFF_SIZE 500

struct grpc_request_t
{
    u64 start_time;
    u64 end_time;
    char method[MAX_SIZE];
    char target[MAX_SIZE];
    struct span_context sc;
    struct span_context psc;
};

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} context_to_grpc_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 clientconn_target_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error
SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke(struct pt_regs *ctx)
{
    // positions
    u64 clientconn_pos = 1;
    u64 context_pos = 3;
    u64 method_ptr_pos = 4;
    u64 method_len_pos = 5;

    struct grpc_request_t grpcReq = {};
    grpcReq.start_time = bpf_ktime_get_ns();

    // Read Method
    void *method_ptr = get_argument(ctx, method_ptr_pos);
    u64 method_len = (u64)get_argument(ctx, method_len_pos);
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Read ClientConn.Target
    void *clientconn_ptr = get_argument(ctx, clientconn_pos);
    void *target_ptr = 0;
    bpf_probe_read(&target_ptr, sizeof(target_ptr), (void *)(clientconn_ptr + (clientconn_target_ptr_pos)));
    u64 target_len = 0;
    bpf_probe_read(&target_len, sizeof(target_len), (void *)(clientconn_ptr + (clientconn_target_ptr_pos + 8)));
    u64 target_size = sizeof(grpcReq.target);
    target_size = target_size < target_len ? target_size : target_len;
    bpf_probe_read(&grpcReq.target, target_size, target_ptr);

    // Write event
    void *context_ptr = get_argument(ctx, context_pos);
    bpf_map_update_elem(&context_to_grpc_events, &context_ptr, &grpcReq, 0);
    return 0;
}

SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke_Returns(struct pt_regs *ctx)
{
    u64 context_pos = 3;
    void *context_ptr = get_argument(ctx, context_pos);
    void *grpcReq_ptr = bpf_map_lookup_elem(&context_to_grpc_events, &context_ptr);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&context_to_grpc_events, &context_ptr);

    return 0;
}
