// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "go_types.h"
#include "go_net.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "uprobe.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_SIZE 100
#define MAX_CONCURRENT 50
#define MAX_HEADERS 20
#define MAX_HEADER_STRING 50

struct grpc_request_t {
    BASE_SPAN_PROPERTIES
    char method[MAX_SIZE];
    u32 status_code;
    net_addr_t local_addr;
    u8 has_status;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} grpc_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, struct grpc_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} streamid_to_grpc_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct grpc_request_t));
    __uint(max_entries, 1);
} grpc_storage_map SEC(".maps");

struct hpack_header_field {
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
volatile const u64 server_stream_stream_pos;
volatile const bool is_new_frame_pos;
volatile const u64 status_s_pos;
volatile const u64 status_code_pos;
volatile const u64 http2server_peer_pos;
volatile const u64 peer_local_addr_pos;

volatile const bool server_addr_supported;

static __always_inline long
dummy_extract_span_context_from_headers(void *stream_id, struct span_context *parent_span_context) {
    return 0;
}

// handleStream handles gRPC stream telemetry.
//
// Arguments:
//   - ctx: the pt_regs passed to the uprobe function
//   - stream_ptr: pointer to the transport.Stream tracking the stream
//   - go_context: the parsed Go context.Context
//
// Returns 0 on success, otherwise a negative error value in case of failure.
static __always_inline int
handleStream(struct pt_regs *ctx, void *stream_ptr, struct go_iface *go_context) {
    if (go_context == NULL) {
        bpf_printk("grpc:server:handleStream: NULL go_context");
        return -1;
    }

    if (stream_ptr == NULL) {
        bpf_printk("grpc:server:handleStream: NULL stream_ptr");
        return -1;
    }

    void *key = (void *)GOROUTINE(ctx);
    void *grpcReq_event_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    if (grpcReq_event_ptr != NULL) {
        bpf_printk("grpc:server:handleStream: event already tracked");
        return 0;
    }

    // Get parent context if exists
    u32 stream_id = 0;
    long rc =
        bpf_probe_read_user(&stream_id, sizeof(stream_id), (void *)(stream_ptr + stream_id_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:handleStream: failed to read stream ID");
        return -2;
    }

    struct grpc_request_t *grpcReq = bpf_map_lookup_elem(&streamid_to_grpc_events, &stream_id);
    if (grpcReq == NULL) {
        // No parent span context, generate new span context
        u32 zero = 0;
        grpcReq = bpf_map_lookup_elem(&grpc_storage_map, &zero);
        if (grpcReq == NULL) {
            bpf_printk("grpc:server:handleStream: failed to get grpcReq");
            return 0;
        }
    }

    grpcReq->start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .sc = &grpcReq->sc,
        .psc = &grpcReq->psc,
        .go_context = go_context,
        // The parent span context is set by operateHeader probe
        .get_parent_span_context_fn = dummy_extract_span_context_from_headers,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Set attributes
    void *method_ptr = stream_ptr + stream_method_ptr_pos;
    bool parsed_method =
        get_go_string_from_user_ptr(method_ptr, grpcReq->method, sizeof(grpcReq->method));
    if (!parsed_method) {
        bpf_printk("grpc:server:handleStream: failed to read gRPC method from stream");
        bpf_map_delete_elem(&streamid_to_grpc_events, &stream_id);
        return -3;
    }

    if (server_addr_supported) {
        void *http2server = get_argument(ctx, 3);
        if (http2server != NULL) {
            void *local_addr_ptr = 0;
            void *local_addr_pos = http2server + http2server_peer_pos + peer_local_addr_pos;
            bpf_probe_read_user(
                &local_addr_ptr, sizeof(local_addr_ptr), get_go_interface_instance(local_addr_pos));
            get_tcp_net_addr_from_tcp_addr(ctx, &grpcReq->local_addr, (void *)(local_addr_ptr));
        } else {
            bpf_printk("grpc:server:handleStream: failed to get http2server arg");
        }
    }

    // Write event
    rc = bpf_map_update_elem(&grpc_events, &key, grpcReq, 0);
    if (rc != 0) {
        bpf_printk("grpc:server:handleStream: failed to update event");
        return -4;
    }
    start_tracking_span(go_context->data, &grpcReq->sc);

    return 0;
}

// writeStatus writes the OTel status to any active spans.
//
// Arguments:
//   - ctx: the pt_regs passed to the uprobe function
//   - status_ptr: pointer to the status.Stream holding the status info
//
// Returns 0 on success, otherwise a negative error value in case of failure.
static __always_inline int writeStatus(struct pt_regs *ctx, void *status_ptr) {
    if (status_ptr == NULL) {
        bpf_printk("grpc:server:writeStatus: NULL status_ptr");
        return -1;
    }

    void *key = (void *)GOROUTINE(ctx);

    struct grpc_request_t *req_ptr = bpf_map_lookup_elem(&grpc_events, &key);
    if (req_ptr == NULL) {
        bpf_printk("grpc:server:handleStream: failed to lookup grpc request");
        return -2;
    }

    void *s_ptr = 0;
    long rc = bpf_probe_read_user(&s_ptr, sizeof(s_ptr), (void *)(status_ptr + status_s_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:handleStream: failed to read Status.s");
        return -3;
    }

    // Get status code from Status.s pointer
    rc = bpf_probe_read_user(
        &req_ptr->status_code, sizeof(req_ptr->status_code), (void *)(s_ptr + status_code_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:handleStream: failed to read status code");
        return -4;
    }
    req_ptr->has_status = true;

    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo)
//
// This is only compatible with versions < 1.69.0 of the Server.
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream(struct pt_regs *ctx) {
    u64 stream_pos = 4;
    void *stream_ptr = get_argument(ctx, stream_pos);
    // Get key
    struct go_iface go_context = {0};
    get_Go_context(ctx, stream_pos, stream_ctx_pos, false, &go_context);

    return handleStream(ctx, stream_ptr, &go_context);
}

UPROBE_RETURN(server_handleStream, struct grpc_request_t, grpc_events)

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.ServerStream)
// https://github.com/grpc/grpc-go/blob/317271b232677b7869576a49855b01b9f4775d67/server.go#L1735
//
// This is only compatible with versions > 1.69.0 of the Server.
SEC("uprobe/server_handleStream2")
int uprobe_server_handleStream2(struct pt_regs *ctx) {
    u64 server_stream_pos = 4;
    void *server_stream_ptr = get_argument(ctx, server_stream_pos);
    if (server_stream_ptr == NULL) {
        bpf_printk("grpc:server:uprobe/server_handleStream2: failed to get ServerStream arg");
        return -1;
    }

    void *stream_ptr;
    long rc = bpf_probe_read_user(
        &stream_ptr, sizeof(stream_ptr), (void *)(server_stream_ptr + server_stream_stream_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:uprobe/server_handleStream2: failed to read stream_ptr");
        return -2;
    }

    struct go_iface go_context = {0};
    rc = bpf_probe_read_user(
        &go_context.type, sizeof(go_context.type), (void *)(stream_ptr + stream_ctx_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:uprobe/server_handleStream2: failed to read context type");
        return -3;
    }

    rc = bpf_probe_read_user(&go_context.data,
                             sizeof(go_context.data),
                             get_go_interface_instance(stream_ptr + stream_ctx_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:uprobe/server_handleStream2: failed to read context data");
        return -4;
    }

    return handleStream(ctx, stream_ptr, &go_context);
}

// This instrumentation attaches a return uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.ServerStream)
// https://github.com/grpc/grpc-go/blob/317271b232677b7869576a49855b01b9f4775d67/server.go#L1735
//
// This is only compatible with versions > 1.69.0 of the Server.
SEC("uprobe/server_handleStream2")
int uprobe_server_handleStream2_Returns(struct pt_regs *ctx) {
    u64 server_stream_pos = 4;
    void *server_stream_ptr = get_argument(ctx, server_stream_pos);
    void *key = NULL;
    if (server_stream_ptr == NULL) {
        // We might fail to get the pointer for versions of Go which use register ABI, as this function does not return anything.
        // This is not an error in that case so we can just go to the lookup which will happen by goroutine.
        goto lookup;
    }

    void *stream_ptr;
    long rc = bpf_probe_read_user(
        &stream_ptr, sizeof(stream_ptr), (void *)(server_stream_ptr + server_stream_stream_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:uprobe/server_handleStream2Return: failed to read stream_ptr");
        return -2;
    }

lookup:
    key = (void *)GOROUTINE(ctx);
    struct grpc_request_t *event = bpf_map_lookup_elem(&grpc_events, &key);
    if (event == NULL) {
        bpf_printk("grpc:server:uprobe/server_handleStream2Return: event is NULL");
        return -5;
    }
    event->end_time = bpf_ktime_get_ns();
    output_span_event(ctx, event, sizeof(struct grpc_request_t), &event->sc);
    stop_tracking_span(&event->sc, &event->psc);
    bpf_map_delete_elem(&grpc_events, &key);
    return 0;
}

// func (d *http2Server) operateHeader(frame *http2.MetaHeadersFrame) error
// for version 1.60 and above:
// func (t *http2Server) operateHeaders(ctx context.Context, frame *http2.MetaHeadersFrame, handle func(*Stream)) error
SEC("uprobe/http2Server_operateHeader")
int uprobe_http2Server_operateHeader(struct pt_regs *ctx) {
    void *arg4 = get_argument(ctx, 4);
    void *arg2 = get_argument(ctx, 2);
    void *frame_ptr = is_new_frame_pos ? arg4 : arg2;
    struct go_slice header_fields = {};
    bpf_probe_read(&header_fields, sizeof(header_fields), (void *)(frame_ptr + frame_fields_pos));
    char key[W3C_KEY_LENGTH] = "traceparent";
    for (s32 i = 0; i < MAX_HEADERS; i++) {
        if (i >= header_fields.len) {
            break;
        }
        struct hpack_header_field hf = {};
        long res =
            bpf_probe_read(&hf, sizeof(hf), (void *)(header_fields.array + (i * sizeof(hf))));
        if (hf.name.len == W3C_KEY_LENGTH && hf.value.len == W3C_VAL_LENGTH) {
            char current_key[W3C_KEY_LENGTH];
            bpf_probe_read(current_key, sizeof(current_key), hf.name.str);
            if (bpf_memcmp(key, current_key, sizeof(key))) {
                char val[W3C_VAL_LENGTH];
                bpf_probe_read(val, W3C_VAL_LENGTH, hf.value.str);

                // Get stream id
                void *headers_frame = NULL;
                bpf_probe_read(&headers_frame, sizeof(headers_frame), frame_ptr);
                u32 stream_id = 0;
                bpf_probe_read(
                    &stream_id, sizeof(stream_id), (void *)(headers_frame + frame_stream_id_pod));
                struct grpc_request_t grpcReq = {};
                w3c_string_to_span_context(val, &grpcReq.psc);
                bpf_map_update_elem(&streamid_to_grpc_events, &stream_id, &grpcReq, 0);
            }
        }
    }

    return 0;
}

// func (ht *http2Server) WriteStatus(s *Stream, st *status.Status)
// https://github.com/grpc/grpc-go/blob/bcf9171a20e44ed81a6eb152e3ca9e35b2c02c5d/internal/transport/http2_server.go#L1049
//
// This is only compatible with versions > 1.40 and < 1.69.0 of the Server.
SEC("uprobe/http2Server_WriteStatus")
int uprobe_http2Server_WriteStatus(struct pt_regs *ctx) {
    void *status_ptr = get_argument(ctx, 3);
    return writeStatus(ctx, status_ptr);
}

// func (ht *http2Server) writeStatus(s *Stream, st *status.Status)
// https://github.com/grpc/grpc-go/blob/317271b232677b7869576a49855b01b9f4775d67/internal/transport/http2_server.go#L1045
//
// This is only compatible with versions > 1.69.0 of the Server.
SEC("uprobe/http2Server_WriteStatus2")
int uprobe_http2Server_WriteStatus2(struct pt_regs *ctx) {
    u64 server_stream_pos = 2;
    void *server_stream_ptr = get_argument(ctx, server_stream_pos);
    if (server_stream_ptr == NULL) {
        bpf_printk("grpc:server:uprobe/http2Server_WriteStatus2: failed to get ServerStream arg");
        return -1;
    }

    void *stream_ptr;
    long rc = bpf_probe_read_user(
        &stream_ptr, sizeof(stream_ptr), (void *)(server_stream_ptr + server_stream_stream_pos));
    if (rc != 0) {
        bpf_printk("grpc:server:uprobe/http2Server_WriteStatus2: failed to read stream_ptr");
        return -2;
    }

    void *status_ptr = get_argument(ctx, 3);
    return writeStatus(ctx, status_ptr);
}
