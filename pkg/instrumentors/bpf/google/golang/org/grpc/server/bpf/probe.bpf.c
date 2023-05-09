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
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 stream_method_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (s *Server) handleStream(t transport.ServerTransport, stream *transport.Stream, trInfo *traceInfo) {
SEC("uprobe/server_handleStream")
int uprobe_server_handleStream(struct pt_regs *ctx)
{
    // Create event
    struct grpc_request_t grpcReq = {};
    grpcReq.start_time = bpf_ktime_get_ns();
    grpcReq.sc = generate_span_context();

    // Get stream pointer
    void *stream_ptr = get_argument(ctx, 4);

    // Get method from stream
    void *method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(stream_ptr + stream_method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(stream_ptr + (stream_method_ptr_pos + 8)));
    u64 method_size = sizeof(grpcReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&grpcReq.method, method_size, method_ptr);

    // Write event
    void *goroutine = get_goroutine_address(ctx, 4);
    bpf_map_update_elem(&context_to_grpc_events, &goroutine, &grpcReq, BPF_ANY);
    bpf_map_update_elem(&spans_in_progress, &goroutine, &grpcReq.sc, BPF_ANY);
    return 0;
}

SEC("uprobe/server_handleStream")
int uprobe_server_handleStream_Returns(struct pt_regs *ctx) {
    void *goroutine = get_goroutine_address(ctx, 4);
    void *grpcReq_ptr = bpf_map_lookup_elem(&context_to_grpc_events, &goroutine);
    struct grpc_request_t grpcReq = {};
    bpf_probe_read(&grpcReq, sizeof(grpcReq), grpcReq_ptr);

    grpcReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &grpcReq, sizeof(grpcReq));
    bpf_map_delete_elem(&context_to_grpc_events, &goroutine);
    bpf_map_delete_elem(&spans_in_progress, &goroutine);
    return 0;
}
