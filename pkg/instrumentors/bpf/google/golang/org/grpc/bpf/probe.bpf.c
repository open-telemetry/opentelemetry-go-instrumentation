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
} grpc_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} streamid_to_grpc_events SEC(".maps");

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
volatile const u64 csattempt_ctx_pos;
volatile const u64 csattempt_stream_pos;
volatile const u64 stream_id_pos;

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

    // Get parent if exists
    void *context_ptr = get_argument(ctx, context_pos);
    void *context_ptr_val = 0;
    bpf_printk("context_ptr: %lx", context_ptr);
    bpf_probe_read(&context_ptr_val, sizeof(context_ptr_val), context_ptr);
    bpf_printk("context_ptr_val: %lx", context_ptr_val);
    struct span_context *parent_span_ctx = get_parent_span_context(context_ptr_val);
    if (parent_span_ctx != NULL)
    {
        bpf_probe_read(&grpcReq.psc, sizeof(grpcReq.psc), parent_span_ctx);
        copy_byte_arrays(grpcReq.psc.TraceID, grpcReq.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(grpcReq.sc.SpanID, SPAN_ID_SIZE);
    }
    else
    {
        grpcReq.sc = generate_span_context();
    }

    // Get key
    void *key = get_consistent_key(ctx, context_ptr);

    // Write event
    bpf_map_update_elem(&grpc_events, &key, &grpcReq, 0);
    return 0;
}

SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke_Returns(struct pt_regs *ctx)
{
    u64 context_pos = 3;
    void *context_ptr = get_argument(ctx, context_pos);
    void *key = get_consistent_key(ctx, context_ptr);
    void *grpcReq_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&grpc_events, &key);
    return 0;
}

// func (l *loopyWriter) writeHeader(streamID uint32, endStream bool, hf []hpack.HeaderField, onWrite func()) error {
SEC("uprobe/loopyWriter_writeHeader")
int uprobe_LoopyWriter_WriterHeader(struct pt_regs *ctx) {
    // TODO(edenfed): loopywriter runs on a different goroutine, so we need to find a way to get the grpcReq
    // We have access to streamID, so we need to have a map to it in previous probes
    // We can replace other probes with:google.golang.org/grpc.(*csAttempt).sendMsg
    bpf_printk("uprobe_LoopyWriter_WriterHeader");
    // Get grpc request struct
    u32 stream_id = get_argument(ctx, 2);
    bpf_printk("stream id at loopyWriter_writeHeader: %d", stream_id);
    void *grpcReq_ptr = bpf_map_lookup_elem(&streamid_to_grpc_events, &stream_id);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);
    bpf_printk("grpcReq.method at loopyWriter_writeHeader: %s", grpcReq.method);
    bpf_map_delete_elem(&streamid_to_grpc_events, &stream_id);
}

SEC("uprobe/csAttempt_sendMsg")
// func (a *csAttempt) sendMsg(m interface{}, hdr, payld, data []byte) error
int uprobe_CsAttempt_SendMsg(struct pt_regs *ctx) {
    bpf_printk("uprobe_CsAttempt_SendMsg");
    s32 csAttempt_pos = 1;
    void *csAttempt_ptr = get_argument(ctx, csAttempt_pos);
    void *context_ptr = 0;
    bpf_probe_read(&context_ptr, sizeof(context_ptr), (void *)(csAttempt_ptr + (csattempt_ctx_pos)));
    bpf_printk("context_ptr: %lx", context_ptr);

    void *stream_ptr = 0;
    bpf_probe_read(&stream_ptr, sizeof(stream_ptr), (void *)(csAttempt_ptr + (csattempt_stream_pos)));
    u32 stream_id = 0;
    bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(stream_ptr + stream_id_pos));
    bpf_printk("stream_id: %d", stream_id);
}

// func (t *http2Client) createHeaderFields(ctx context.Context, callHdr *CallHdr) ([]hpack.HeaderField, error)
SEC("uprobe/Http2Client_createHeaderFields")
int uprobe_Http2Client_CreateHeaderFields(struct pt_regs *ctx)
{
    // TODO: Delete this probe, do the writing in loopyWriter_writeHeader
    // Read slice
    s32 context_pointer_pos = 3;
    struct go_slice slice = {};
    struct go_slice_user_ptr slice_user_ptr = {};
    void *context_ptr = get_argument(ctx, context_pointer_pos);
    void *key = get_consistent_key(ctx, context_ptr);
    if (is_registers_abi)
    {
        slice.array = (void *)GO_PARAM1(ctx);
        slice.len = (s32)GO_PARAM2(ctx);
        slice.cap = (s32)GO_PARAM3(ctx);
        slice_user_ptr.array = (void *)GO_PARAM1(ctx);
        bpf_printk("slice_user_ptr.array: %lx", slice_user_ptr.array);
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
        bpf_printk("key %lx", key);
        key = get_parent_go_context(key, &grpc_events);
    }
    char tp_key[11] = "traceparent";
    struct go_string key_str = write_user_go_string(tp_key, sizeof(tp_key));
    if (key_str.len == 0) {
        bpf_printk("write failed, aborting ebpf probe");
        return 0;
    }

    // Get grpc request struct
    void *grpcReq_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    // Write headers
    char val[SPAN_CONTEXT_STRING_SIZE];
    span_context_to_w3c_string(&grpcReq.sc, val);
    struct go_string val_str = write_user_go_string(val, sizeof(val));
    bpf_printk("writing string %s to address %lx", val, val_str.str);
    struct hpack_header_field hf = {};
    hf.name = key_str;
    hf.value = val_str;
    append_item_to_slice(&slice, &hf, sizeof(hf), &slice_user_ptr, &headers_buff_map);
    bpf_map_update_elem(&grpc_events, &key, &grpcReq, 0);

    return 0;
}