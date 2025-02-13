// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "go_types.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "uprobe.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 50
#define MAX_CONCURRENT 50
#define MAX_ERROR_LEN 128

struct grpc_request_t
{
    BASE_SPAN_PROPERTIES
    char err_msg[MAX_ERROR_LEN];
    char method[MAX_SIZE];
    char target[MAX_SIZE];
    u32 status_code;
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
    __type(value, struct span_context);
    __uint(max_entries, MAX_CONCURRENT);
} streamid_to_span_contexts SEC(".maps");

// Injected in init
volatile const u64 clientconn_target_ptr_pos;
volatile const u64 httpclient_nextid_pos;
volatile const u64 headerFrame_streamid_pos;
volatile const u64 headerFrame_hf_pos;
volatile const u64 error_status_pos;
volatile const u64 status_s_pos;
volatile const u64 status_message_pos;
volatile const u64 status_code_pos;

volatile const bool write_status_supported;

// This instrumentation attaches uprobe to the following function:
// func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error
SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke(struct pt_regs *ctx)
{
    // positions
    u64 clientconn_pos = 1;
    u64 method_ptr_pos = 4;
    u64 method_len_pos = 5;

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);

    // Get key
    void *key = get_consistent_key(ctx);
    void *grpcReq_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    if (grpcReq_ptr != NULL)
    {
        bpf_printk("uprobe/ClientConn_Invoke already tracked with the current context");
        return 0;
    }

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
    if (!get_go_string_from_user_ptr((void*)(clientconn_ptr + clientconn_target_ptr_pos), grpcReq.target, sizeof(grpcReq.target)))
    {
        bpf_printk("target write failed, aborting ebpf probe");
        return 0;
    }

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &grpcReq.psc,
        .sc = &grpcReq.sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Write event
    bpf_map_update_elem(&grpc_events, &key, &grpcReq, 0);
    start_tracking_span(go_context.data, &grpcReq.sc);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error
SEC("uprobe/ClientConn_Invoke")
int uprobe_ClientConn_Invoke_Returns(struct pt_regs *ctx) {
    void *key = get_consistent_key(ctx);
    struct grpc_request_t *grpc_span = bpf_map_lookup_elem(&grpc_events, &key);
    if (grpc_span == NULL) {
        bpf_printk("event is NULL in ret probe");
        return 0;
    }

    if(!write_status_supported) {
        goto done;
    }
    // Getting the returned response (error)
    // The status code is embedded 3 layers deep:
    // Invoke() error
    // the `error` interface concrete type here is a gRPC `internal.Error` struct
    // type Error struct {
    //   s *Status
    // }
    // The `Error` struct embeds a `Status` proto object
    // type Status struct {
    //   s *Status
    // }
    // The `Status` proto object contains a `Code` int32 field, which is what we want
    // type Status struct {
    //     Code int32
    //     Message string
    //     Details []*anypb.Any
    // }
    void *resp_ptr = get_argument(ctx, 2);
    if(resp_ptr == 0) {
        // err == nil
        goto done;
    }
    void *status_ptr = 0;
    // get `s` (Status pointer field) from Error struct
    bpf_probe_read_user(&status_ptr, sizeof(status_ptr), (void *)(resp_ptr+error_status_pos));
    // get `s` field from Status object pointer
    void *s_ptr = 0;
    bpf_probe_read_user(&s_ptr, sizeof(s_ptr), (void *)(status_ptr + status_s_pos));
    // Get status code from Status.s pointer
    bpf_probe_read_user(&grpc_span->status_code, sizeof(grpc_span->status_code), (void *)(s_ptr + status_code_pos));
    get_go_string_from_user_ptr((void *)(s_ptr + status_message_pos), grpc_span->err_msg, sizeof(grpc_span->err_msg));

done:
    grpc_span->end_time = bpf_ktime_get_ns();
    output_span_event(ctx, grpc_span, sizeof(*grpc_span), &grpc_span->sc);
    stop_tracking_span(&grpc_span->sc, &grpc_span->psc);
    bpf_map_delete_elem(&grpc_events, &key);
    return 0;
}

// func (l *loopyWriter) headerHandler(h *headerFrame) error
SEC("uprobe/loopyWriter_headerHandler")
int uprobe_LoopyWriter_HeaderHandler(struct pt_regs *ctx)
{
    void *headerFrame_ptr = get_argument(ctx, 2);
    u32 stream_id = 0;
    bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(headerFrame_ptr + (headerFrame_streamid_pos)));
    void *sc_ptr = bpf_map_lookup_elem(&streamid_to_span_contexts, &stream_id);
    if (sc_ptr == NULL)
    {
        return 0;
    }

    struct span_context current_span_context = {};
    bpf_probe_read(&current_span_context, sizeof(current_span_context), sc_ptr);

    char tp_key[11] = "traceparent";
    struct go_string key_str = write_user_go_string(tp_key, sizeof(tp_key));
    if (key_str.len == 0) {
        bpf_printk("key write failed, aborting ebpf probe");
        goto done;
    }

    // Write headers
    char val[SPAN_CONTEXT_STRING_SIZE];
    span_context_to_w3c_string(&current_span_context, val);
    struct go_string val_str = write_user_go_string(val, sizeof(val));
    if (val_str.len == 0) {
        bpf_printk("val write failed, aborting ebpf probe");
        goto done;
    }
    struct hpack_header_field hf = {};
    hf.name = key_str;
    hf.value = val_str;
    append_item_to_slice(&hf, sizeof(hf), (void *)(headerFrame_ptr + (headerFrame_hf_pos)));
done:
    bpf_map_delete_elem(&streamid_to_span_contexts, &stream_id);

    return 0;
}

SEC("uprobe/http2Client_NewStream")
// func (t *http2Client) NewStream(ctx context.Context, callHdr *CallHdr) (*Stream, error)
int uprobe_http2Client_NewStream(struct pt_regs *ctx)
{
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    void *httpclient_ptr = get_argument(ctx, 1);
    u32 nextid = 0;
    bpf_probe_read(&nextid, sizeof(nextid), (void *)(httpclient_ptr + (httpclient_nextid_pos)));
    // Get the span context from go context. The mapping is created in the Invoke probe,
    // the context here is derived from the Invoke context.
    struct span_context *current_span_context = get_parent_span_context(&go_context);
    if (current_span_context != NULL) {
        bpf_map_update_elem(&streamid_to_span_contexts, &nextid, current_span_context, 0);
    }

    return 0;
}
