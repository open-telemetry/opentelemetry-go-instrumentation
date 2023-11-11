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
#include "span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "otel_types.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_CONCURRENT 50
#define MAX_SPAN_NAME_LEN 64
#define MAX_ATTRIBUTES 4

struct otel_span_t {
    BASE_SPAN_PROPERTIES
    char span_name[MAX_SPAN_NAME_LEN];
    // otel_attribute_t attributes[MAX_ATTRIBUTES];
    otel_attributes_t attributes;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct otel_span_t);
	__uint(max_entries, MAX_CONCURRENT);
} active_spans SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct otel_span_t));
    __uint(max_entries, 1);
} otel_span_storage_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 span_name_pos;
volatile const u64 span_attributes_pos;

// This instrumentation attaches uprobe to the following function:
// func (tr *tracer) Start(ctx context.Context, name string, options ...trace.SpanStartOption) (context.Context, trace.Span)
SEC("uprobe/Start")
int uprobe_Start_Returns(struct pt_regs *ctx) {
    u32 map_key = 0;
    struct otel_span_t *otel_span = bpf_map_lookup_elem(&otel_span_storage_map, &map_key);
    if (otel_span == NULL) {
        return 0;
    }
    bpf_memset((unsigned char *)otel_span, 0, sizeof(*otel_span));
    otel_span->start_time = bpf_ktime_get_ns();

    // Get the ** returned ** context and Span (concrete type of the interfaces)
    void *context_ptr_val = get_argument(ctx, 2);
    void *span_ptr_val = get_argument(ctx, 4);

    struct span_context *span_ctx = get_parent_span_context(context_ptr_val);
    if (span_ctx != NULL) {
        // Set the parent context
        bpf_probe_read(&otel_span->psc, sizeof(otel_span->psc), span_ctx);
        copy_byte_arrays(otel_span->psc.TraceID, otel_span->sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(otel_span->sc.SpanID, SPAN_ID_SIZE);
    } else {
        otel_span->sc = generate_span_context();
    }

    bpf_map_update_elem(&active_spans, &span_ptr_val, otel_span, 0);
    start_tracking_span(context_ptr_val, &otel_span->sc);
    return 0;
}


// This instrumentation attaches uprobe to the following function:
// unc (s *recordingSpan) End(options ...trace.SpanEndOption)
SEC("uprobe/End")
int uprobe_End(struct pt_regs *ctx) {
    void *recording_span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans, &recording_span_ptr);
    if (span == NULL) {
        return 0;
    }
    span->end_time = bpf_ktime_get_ns();
    bpf_map_delete_elem(&active_spans, &recording_span_ptr);
    stop_tracking_span(&span->sc, &span->psc);

    if (get_go_string_from_user_ptr((void *)(recording_span_ptr + span_name_pos), span->span_name, sizeof(span->span_name)) < 0) {
        bpf_printk("failed to get span name from manual span");
        return 0;
    }

    convert_go_otel_attributes((void *)(recording_span_ptr + span_attributes_pos), &span->attributes);
    //convert_attributes_slice((void *)(recording_span_ptr + span_attributes_pos), span->attributes, MAX_ATTRIBUTES);

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, span, sizeof(*span));
    return 0;
}