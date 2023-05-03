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

struct hpack_header_field
{
    struct go_string name;
    struct go_string value;
    bool sensitive;
};

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} context_to_grpc_events SEC(".maps");

struct headers_buff
{
    unsigned char buff[MAX_HEADERS_BUFF_SIZE];
};

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, s32);
    __type(value, struct headers_buff);
    __uint(max_entries, 1);
} headers_buff_map SEC(".maps");

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

// func (t *http2Client) createHeaderFields(ctx context.Context, callHdr *CallHdr) ([]hpack.HeaderField, error)
SEC("uprobe/Http2Client_createHeaderFields")
int uprobe_Http2Client_CreateHeaderFields(struct pt_regs *ctx)
{
    // Read slice
    s32 context_pointer_pos = 3;
    struct go_slice slice = {};
    struct go_slice_user_ptr slice_user_ptr = {};
    if (is_registers_abi)
    {
        slice.array = (void *)GO_PARAM1(ctx);
        slice.len = (s32)GO_PARAM2(ctx);
        slice.cap = (s32)GO_PARAM3(ctx);
        slice_user_ptr.array = (void *)&GO_PARAM1(ctx);
        slice_user_ptr.len = (void *)&GO_PARAM2(ctx);
        slice_user_ptr.cap = (void *)&GO_PARAM3(ctx);
    }
    else
    {
        u64 slice_pointer_pos = 5;
        s32 slice_len_pos = 6;
        s32 slice_cap_pos = 7;
        slice.array = get_argument(ctx, slice_pointer_pos);
        slice.len = (long)get_argument(ctx, slice_len_pos);
        slice.cap = (long)get_argument(ctx, slice_cap_pos);
        slice_user_ptr.array = (void *)(PT_REGS_SP(ctx) + (slice_pointer_pos * 8));
        slice_user_ptr.len = (void *)(PT_REGS_SP(ctx) + (slice_len_pos * 8));
        slice_user_ptr.cap = (void *)(PT_REGS_SP(ctx) + (slice_cap_pos * 8));
    }
    char key[11] = "traceparent";
    struct go_string key_str = write_user_go_string(key, sizeof(key));
    if (key_str.len == 0) {
        bpf_printk("write failed, aborting ebpf probe");
        return 0;
    }

    // Get grpc request struct
    void *context_ptr = 0;
    bpf_probe_read(&context_ptr, sizeof(context_ptr), (void *)(PT_REGS_SP(ctx) + (context_pointer_pos * 8)));
    void *parent_ctx = find_context_in_map(context_ptr, &context_to_grpc_events);
    void *grpcReq_ptr = bpf_map_lookup_elem(&context_to_grpc_events, &parent_ctx);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    // Get parent if exists
    void *parent_span_ctx = find_context_in_map(context_ptr, &spans_in_progress);
    if (parent_span_ctx != NULL)
    {
        void *psc_ptr = bpf_map_lookup_elem(&spans_in_progress, &parent_span_ctx);
        bpf_probe_read(&grpcReq.psc, sizeof(grpcReq.psc), psc_ptr);
        copy_byte_arrays(grpcReq.psc.TraceID, grpcReq.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(grpcReq.sc.SpanID, SPAN_ID_SIZE);
    }
    else
    {
        grpcReq.sc = generate_span_context();
    }

    // Write headers
    char val[SPAN_CONTEXT_STRING_SIZE];
    span_context_to_w3c_string(&grpcReq.sc, val);
    struct go_string val_str = write_user_go_string(val, sizeof(val));
    struct hpack_header_field hf = {};
    hf.name = key_str;
    hf.value = val_str;
    append_item_to_slice(&slice, &hf, sizeof(hf), &slice_user_ptr, &headers_buff_map);
    bpf_map_update_elem(&context_to_grpc_events, &parent_ctx, &grpcReq, 0);

    return 0;
}