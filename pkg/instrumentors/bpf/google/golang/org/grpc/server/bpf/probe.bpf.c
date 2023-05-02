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

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100
#define MAX_CONCURRENT 50
#define MAX_HEADERS 20
#define MAX_HEADER_STRING 50
#define W3C_KEY_LENGTH 11
#define W3C_VAL_LENGTH 55

struct grpc_request_t
{
    u64 start_time;
    u64 end_time;
    char method[MAX_SIZE];
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
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} streamid_to_grpc_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

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

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo) {
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream(struct pt_regs *ctx)
{
    u64 stream_pos = 4;
    void *stream_ptr = get_argument(ctx, stream_pos);

    // Get parent context if exists
    u32 stream_id = 0;
    bpf_probe_read(&stream_id, sizeof(stream_id), (void *)(stream_ptr + stream_id_pos));
    void *grpcReq_ptr = bpf_map_lookup_elem(&streamid_to_grpc_events, &stream_id);
    struct grpc_request_t grpcReq = {};
    if (grpcReq_ptr != NULL)
    {
        bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);
        bpf_map_delete_elem(&streamid_to_grpc_events, &stream_id);
        copy_byte_arrays(grpcReq.psc.TraceID, grpcReq.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(grpcReq.sc.SpanID, SPAN_ID_SIZE);
    }
    else
    {
        grpcReq.sc = generate_span_context();
    }

    // Set attributes
    grpcReq.start_time = bpf_ktime_get_ns();
    void *method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr + stream_method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr + (stream_method_ptr_pos + 8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Write event
    void *ctx_iface = 0;
    bpf_probe_read(&ctx_iface, sizeof(ctx_iface), (void *)(stream_ptr + stream_ctx_pos));
    void *ctx_instance = 0;
    bpf_probe_read(&ctx_instance, sizeof(ctx_instance), (void *)(ctx_iface + 8));
    bpf_map_update_elem(&context_to_grpc_events, &ctx_instance, &grpcReq, 0);
    bpf_map_update_elem(&spans_in_progress, &ctx_instance, &grpcReq.sc, 0);
    return 0;
}

SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_Returns(struct pt_regs *ctx) {
    u64 stream_pos = 4;
    void *stream_ptr = get_argument(ctx, stream_pos);
    void *ctx_iface = 0;
    bpf_probe_read(&ctx_iface, sizeof(ctx_iface), (void *)(stream_ptr + stream_ctx_pos));
    void *ctx_instance = 0;
    bpf_probe_read(&ctx_instance, sizeof(ctx_instance), (void *)(ctx_iface + 8));

    void *grpcReq_ptr = bpf_map_lookup_elem(&context_to_grpc_events, &ctx_instance);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&context_to_grpc_events, &ctx_instance);
    bpf_map_delete_elem(&spans_in_progress, &ctx_instance);
    return 0;
}

// func (d *decodeState) decodeHeader(frame *http2.MetaHeadersFrame) error
SEC("uprobe/decodeState_decodeHeader")
int uprobe_decodeState_decodeHeader(struct pt_regs *ctx)
{
    u64 frame_pos = 2;
    void *frame_ptr = get_argument(ctx, frame_pos);
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