// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "go_types.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "uprobe.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100
#define MAX_CONCURRENT 50
#define MAX_HEADERS 20
#define MAX_HEADER_STRING 50

struct grpc_request_t
{
    BASE_SPAN_PROPERTIES
    char method[MAX_SIZE];
    u32 status_code;
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

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct grpc_request_t));
    __uint(max_entries, 1);
} grpc_storage_map SEC(".maps");

struct hpack_header_field
{
    struct go_string name;
    struct go_string value;
    bool sensitive;
};

// Injected in init
volatile const u64 stream_method_ptr_pos;
volatile const u64 frame_fields_pos;
volatile const u64 frame_stream_id_pod;
volatile const u64 stream_id_pos;
volatile const u64 stream_ctx_pos;
volatile const bool is_new_frame_pos;
volatile const u64 status_s_pos;
volatile const u64 status_code_pos;

static __always_inline long dummy_extract_span_context_from_headers(void *stream_id, struct span_context *parent_span_context) {
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo)
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream(struct pt_regs *ctx)
{
    u64 stream_pos = 4;
    void *stream_ptr = get_argument(ctx, stream_pos);
    // Get key
    struct go_iface go_context = {0};
    get_Go_context(ctx, 4, stream_ctx_pos, false, &go_context);
    void *key = get_consistent_key(ctx, go_context.data);
    void *grpcReq_event_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    if (grpcReq_event_ptr != NULL)
    {
        bpf_printk("uprobe/server_handleStream already tracked with the current context");
        return 0;
    }

    // Get parent context if exists
    u32 stream_id = 0;
    bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(stream_ptr + stream_id_pos));
    struct grpc_request_t *grpcReq = bpf_map_lookup_elem(&streamid_to_grpc_events, &stream_id);
    if (grpcReq == NULL) {
        // No parent span context, generate new span context
        u32 zero = 0;
        grpcReq = bpf_map_lookup_elem(&grpc_storage_map, &zero);
        if (grpcReq == NULL) {
            bpf_printk("failed to get grpcReq from storage map");
            return 0;
        }
    }

    grpcReq->start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .sc = &grpcReq->sc,
        .psc = &grpcReq->psc,
        .go_context = &go_context,
        // The parent span context is set by operateHeader probe
        .get_parent_span_context_fn = dummy_extract_span_context_from_headers,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Set attributes
    if (!get_go_string_from_user_ptr((void *)(stream_ptr + stream_method_ptr_pos), grpcReq->method, sizeof(grpcReq->method)))
    {
        bpf_printk("Failed to read gRPC method from stream");
        bpf_map_delete_elem(&streamid_to_grpc_events, &stream_id);
        return 0;
    }

    // Write event
    bpf_map_update_elem(&grpc_events, &key, grpcReq, 0);
    start_tracking_span(go_context.data, &grpcReq->sc);

    return 0;
}

UPROBE_RETURN(server_handleStream, struct grpc_request_t, grpc_events, events, 4, stream_ctx_pos, false)

// func (d *http2Server) operateHeader(frame *http2.MetaHeadersFrame) error
// for version 1.60 and above:
// func (t *http2Server) operateHeaders(ctx context.Context, frame *http2.MetaHeadersFrame, handle func(*Stream)) error
SEC("uprobe/http2Server_operateHeader")
int uprobe_http2Server_operateHeader(struct pt_regs *ctx)
{
    void *frame_ptr = is_new_frame_pos ? get_argument(ctx, 4) : get_argument(ctx, 2);
    struct go_slice header_fields = {};
    bpf_probe_read(&header_fields, sizeof(header_fields), (void *)(frame_ptr + frame_fields_pos));
    char key[W3C_KEY_LENGTH] = "traceparent";
    for (s32 i = 0; i < MAX_HEADERS; i++)
    {
        if (i >= header_fields.len)
        {
            break;
        }
        struct hpack_header_field hf = {};
        long res = bpf_probe_read(&hf, sizeof(hf), (void *)(header_fields.array + (i * sizeof(hf))));
        if (hf.name.len == W3C_KEY_LENGTH && hf.value.len == W3C_VAL_LENGTH)
        {
            char current_key[W3C_KEY_LENGTH];
            bpf_probe_read(current_key, sizeof(current_key), hf.name.str);
            if (bpf_memcmp(key, current_key, sizeof(key)))
            {
                char val[W3C_VAL_LENGTH];
                bpf_probe_read(val, W3C_VAL_LENGTH, hf.value.str);

                // Get stream id
                void *headers_frame = NULL;
                bpf_probe_read(&headers_frame, sizeof(headers_frame), frame_ptr);
                u32 stream_id = 0;
                bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(headers_frame + frame_stream_id_pod));
                struct grpc_request_t grpcReq = {};
                w3c_string_to_span_context(val, &grpcReq.psc);
                bpf_map_update_elem(&streamid_to_grpc_events, &stream_id, &grpcReq, 0);
            }
        }
    }

    return 0;
}

static __always_inline int get_status_code(struct pt_regs *ctx) {
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, stream_ctx_pos, true, &go_context);
    void *key = get_consistent_key(ctx, go_context.data);

    // Get parent context if exists
    void *stream_ptr = get_argument(ctx, 2);
    u32 stream_id = 0;
    bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(stream_ptr + stream_id_pos));
    struct grpc_request_t *grpcReq = bpf_map_lookup_elem(&streamid_to_grpc_events, &stream_id);
    if (grpcReq == NULL) {
        // No parent span context, generate new span context
        u32 zero = 0;
        grpcReq = bpf_map_lookup_elem(&grpc_storage_map, &zero);
        if (grpcReq == NULL) {
            bpf_printk("failed to get grpcReq from storage map");
            return 0;
        }
    }

    void *status_ptr = get_argument(ctx, 3);
    void *s_ptr = 0;
    bpf_probe_read_user(&s_ptr, sizeof(s_ptr), (void *)(status_ptr + status_s_pos));
    // Get status code from Status.s pointer
    bpf_probe_read_user(&grpcReq->status_code, sizeof(grpcReq->status_code), (void *)(s_ptr + status_code_pos));

    bpf_map_update_elem(&grpc_events, &key, grpcReq, 0);
    bpf_map_delete_elem(&streamid_to_grpc_events, &stream_id);
    return 0;
}

// func (ht *serverHandlerTransport) WriteStatus(s *Stream, st *status.Status)
// https://github.com/grpc/grpc-go/blob/bcf9171a20e44ed81a6eb152e3ca9e35b2c02c5d/internal/transport/handler_server.go#L228
SEC("uprobe/serverHandlerTransport_WriteStatus")
int uprobe_serverHandlerTransport_WriteStatus(struct pt_regs *ctx) {
    return get_status_code(ctx);
}

// func (ht *serverHandlerTransport) WriteStatus(s *Stream, st *status.Status)
// https://github.com/grpc/grpc-go/blob/bcf9171a20e44ed81a6eb152e3ca9e35b2c02c5d/internal/transport/http2_server.go#L1049
SEC("uprobe/http2Server_WriteStatus")
int uprobe_http2Server_WriteStatus(struct pt_regs *ctx) {
    return get_status_code(ctx);
}
