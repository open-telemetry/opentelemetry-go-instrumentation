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
#include "span_output.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_ATTRIBUTES 4
#define MAX_CONCURRENT 50
#define MAX_SPAN_NAME_LEN 64
#define MAX_STATUS_DESCRIPTION_LEN 64

struct span_description_t {
    char buf[MAX_STATUS_DESCRIPTION_LEN];
};

typedef struct otel_status {
	u32 code;
	struct span_description_t description;
} __attribute__((packed)) otel_status_t;

struct span_name_t {
    char buf[MAX_SPAN_NAME_LEN];
};

struct otel_span_t {
    BASE_SPAN_PROPERTIES
    struct span_name_t span_name;
    otel_status_t status;
    otel_attributes_t attributes;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct otel_span_t);
	__uint(max_entries, MAX_CONCURRENT);
} active_spans_by_span_ptr SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct span_name_t);
	__uint(max_entries, MAX_CONCURRENT);
} span_name_by_context SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct otel_span_t));
    __uint(max_entries, 2);
} otel_span_storage_map SEC(".maps");

// Injected in init
volatile const u64 tracer_delegate_pos;

// read_span_name reads the span name from the provided span_name_ptr and stores the result in
// span_name.buf.
static __always_inline void read_span_name(struct span_name_t *span_name, const u64 span_name_len, void *span_name_ptr) {
    const u64 span_name_size = MAX_SPAN_NAME_LEN < span_name_len ? MAX_SPAN_NAME_LEN : span_name_len;
    bpf_probe_read(span_name->buf, span_name_size, span_name_ptr);
}

// This instrumentation attaches uprobe to the following function:
// func (t *tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
// https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/internal/global/trace.go#L149
SEC("uprobe/Start")
int uprobe_Start(struct pt_regs *ctx) {
    void *tracer_ptr = get_argument(ctx, 1);
    void *delegate_ptr = NULL;
    bpf_probe_read(&delegate_ptr, sizeof(delegate_ptr), (void*)(tracer_ptr + tracer_delegate_pos));
    if (delegate_ptr != NULL) {
        // Delegate is set, so we should not instrument this call
        return 0;
    }
    struct span_name_t span_name = {0};

    // Getting span name
    void *span_name_ptr = get_argument(ctx, 4);
    u64 span_name_len = (u64)get_argument(ctx, 5);
    read_span_name(&span_name, span_name_len, span_name_ptr);

    // Save the span name in map to be read once the Start function returns
    void *context_ptr_val = get_Go_context(ctx, 3, 0, true);
    void *key = get_consistent_key(ctx, context_ptr_val);
    bpf_map_update_elem(&span_name_by_context, &key, &span_name, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (t *tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
// https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/internal/global/trace.go#L149
SEC("uprobe/Start")
int uprobe_Start_Returns(struct pt_regs *ctx) {
    // Get the span name passed to the Start function
    void *context_ptr_val = get_Go_context(ctx, 3, 0, true);
    void *key = get_consistent_key(ctx, context_ptr_val);
    struct span_name_t *span_name = bpf_map_lookup_elem(&span_name_by_context, &key); 
    if (span_name == NULL) {
        return 0;
    }

    u32 zero_span_key = 0;
    struct otel_span_t *zero_span = bpf_map_lookup_elem(&otel_span_storage_map, &zero_span_key);
    if (zero_span == NULL) {
        goto done;
    }

    u32 otel_span_key = 1;
    // Zero the span we are about to build, eBPF doesn't support memset of large structs (more than 1024 bytes)
    bpf_map_update_elem(&otel_span_storage_map, &otel_span_key, zero_span, 0);
    // Get a pointer to the zeroed span
    struct otel_span_t *otel_span = bpf_map_lookup_elem(&otel_span_storage_map, &otel_span_key);
    if (otel_span == NULL) {
        goto done;
    }

    otel_span->start_time = bpf_ktime_get_ns();
    copy_byte_arrays((unsigned char*)span_name->buf, (unsigned char*)otel_span->span_name.buf, MAX_SPAN_NAME_LEN);

    // Get the ** returned ** context and Span (concrete type of the interfaces)
    void *ret_context_ptr_val = get_argument(ctx, 2);
    void *span_ptr_val = get_argument(ctx, 4);

    struct span_context *span_ctx = get_parent_span_context(ret_context_ptr_val);
    if (span_ctx != NULL) {
        // Set the parent context
        bpf_probe_read(&otel_span->psc, sizeof(otel_span->psc), span_ctx);
        get_span_context_from_parent(&otel_span->psc, &otel_span->sc);
    } else {
        get_root_span_context(&otel_span->sc);
    }

    bpf_map_update_elem(&active_spans_by_span_ptr, &span_ptr_val, otel_span, 0);
    start_tracking_span(ret_context_ptr_val, &otel_span->sc);

done:
    bpf_map_delete_elem(&span_name_by_context, &key);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (*nonRecordingSpan) SetAttributes(...attribute.KeyValue)
SEC("uprobe/SetAttributes")
int uprobe_SetAttributes(struct pt_regs *ctx) {
    void *non_recording_span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans_by_span_ptr, &non_recording_span_ptr);
    if (span == NULL) {
        return 0;
    }

    // In Go, "..." is equivalent to passing a slice: https://go.dev/ref/spec#Passing_arguments_to_..._parameters
    void *attributes_usr_buf = get_argument(ctx, 2);
    u64 attributes_len = (u64)get_argument(ctx, 3);
    convert_go_otel_attributes(attributes_usr_buf, attributes_len, &span->attributes);

    // Update the map entry with the new attributes
    bpf_map_update_elem(&active_spans_by_span_ptr, &non_recording_span_ptr, span, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (nonRecordingSpan) SetName(string)
SEC("uprobe/SetName")
int uprobe_SetName(struct pt_regs *ctx) {
    void *non_recording_span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans_by_span_ptr, &non_recording_span_ptr);
    if (span == NULL) {
        return 0;
    }

    void *span_name_ptr = get_argument(ctx, 2);
    if (span_name_ptr == NULL) {
        return 0;
    }

    void *span_name_len_ptr = get_argument(ctx, 3);
    if (span_name_len_ptr == NULL) {
        return 0;
    }

    u64 span_name_len = (u64)span_name_len_ptr;
    struct span_name_t span_name = {0};

    read_span_name(&span_name, span_name_len, span_name_ptr);
    span->span_name = span_name;
    bpf_map_update_elem(&active_spans_by_span_ptr, &non_recording_span_ptr, span, 0);
    
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (nonRecordingSpan) SetStatus(codes.Code, string)
SEC("uprobe/SetStatus")
int uprobe_SetStatus(struct pt_regs *ctx) {
    void *non_recording_span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans_by_span_ptr, &non_recording_span_ptr);
    if (span == NULL) {
        return 0;
    }

    u64 status_code = (u64)get_argument(ctx, 2);

    void *description_ptr = get_argument(ctx, 3);
    if (description_ptr == NULL) {
        return 0;
    }

    struct span_description_t description = {0};

    // Getting span description
    u64 description_len = (u64)get_argument(ctx, 4);
    u64 description_size = MAX_STATUS_DESCRIPTION_LEN < description_len ? MAX_STATUS_DESCRIPTION_LEN : description_len;
    bpf_probe_read(description.buf, description_size, description_ptr);

    otel_status_t status = {0};

    status.code = (u32)status_code;
    status.description = description;
    span->status = status;
    bpf_map_update_elem(&active_spans_by_span_ptr, &non_recording_span_ptr, span, 0);

    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (*nonRecordingSpan) End(...trace.SpanEndOption)
SEC("uprobe/End")
int uprobe_End(struct pt_regs *ctx) {
    void *non_recording_span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans_by_span_ptr, &non_recording_span_ptr);
    if (span == NULL) {
        return 0;
    }
    span->end_time = bpf_ktime_get_ns();
    stop_tracking_span(&span->sc, &span->psc);

    output_span_event(ctx, span, sizeof(*span), &span->sc);

    bpf_map_delete_elem(&active_spans_by_span_ptr, &non_recording_span_ptr);
    return 0;
}
