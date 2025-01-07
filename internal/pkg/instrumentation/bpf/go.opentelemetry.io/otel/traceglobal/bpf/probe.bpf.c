// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "otel_types.h"
#include "trace/span_output.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_ATTRIBUTES 4
#define MAX_CONCURRENT 50
#define MAX_SPAN_NAME_LEN 64
#define MAX_STATUS_DESCRIPTION_LEN 64
#define MAX_TRACER_NAME_LEN 128
#define MAX_TRACER_VERSION_LEN 32
#define MAX_TRACER_SCHEMA_URL_LEN 128

#define MAX_BUCKETS 8
#define MAX_TRACERS 64

// Records state of our write to auto-instrumentation flag.
bool wrote_flag = false;

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

typedef struct tracer_id {
    char name[MAX_TRACER_NAME_LEN];
    char version[MAX_TRACER_VERSION_LEN];
    char schema_url[MAX_TRACER_SCHEMA_URL_LEN];
} tracer_id_t;

struct control_t {
    u32 kind; // Required to be 1.
};

struct otel_span_t {
    u32 kind; // Required to be 0.
    BASE_SPAN_PROPERTIES
    struct span_name_t span_name;
    otel_status_t status;
    otel_attributes_t attributes;
    tracer_id_t tracer_id;
};

typedef struct go_tracer_id_partial {
    struct go_string name;
    struct go_string version;
} go_tracer_id_partial_t;

typedef struct go_tracer_with_schema {
    struct go_string name;
    struct go_string version;
    struct go_string schema_url;
} go_tracer_with_schema_t;

typedef struct go_tracer_with_scope_attributes {
    struct go_string name;
    struct go_string version;
    struct go_string schema_url;
    go_iface_t scope_attributes;
} go_tracer_with_scope_attributes_t;


typedef void* go_tracer_ptr; 

// tracerProvider contains a map of tracers
MAP_BUCKET_DEFINITION(go_tracer_id_partial_t, go_tracer_ptr)
MAP_BUCKET_DEFINITION(go_tracer_with_schema_t, go_tracer_ptr)
MAP_BUCKET_DEFINITION(go_tracer_with_scope_attributes_t, go_tracer_ptr)

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

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, tracer_id_t);
	__uint(max_entries, MAX_CONCURRENT);
} tracer_id_by_context SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct otel_span_t));
    __uint(max_entries, 2);
} otel_span_storage_map SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(MAP_BUCKET_TYPE(go_tracer_with_scope_attributes_t, go_tracer_ptr)));
    __uint(max_entries, 1);
} golang_mapbucket_storage_map SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(tracer_id_t));
    __uint(max_entries, 1);
} tracer_id_storage_map SEC(".maps");


struct
{
   	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, tracer_id_t);
	__uint(max_entries, MAX_TRACERS);
} tracer_ptr_to_id_map SEC(".maps");


// Injected in init
volatile const u64 tracer_delegate_pos;
volatile const u64 tracer_name_pos;
volatile const u64 tracer_provider_pos;
volatile const u64 tracer_provider_tracers_pos;
volatile const u64 buckets_ptr_pos;

volatile const bool tracer_id_contains_schemaURL;
volatile const bool tracer_id_contains_scope_attributes;

// read_span_name reads the span name from the provided span_name_ptr and stores the result in
// span_name.buf.
static __always_inline void read_span_name(struct span_name_t *span_name, const u64 span_name_len, void *span_name_ptr) {
    const u64 span_name_size = MAX_SPAN_NAME_LEN < span_name_len ? MAX_SPAN_NAME_LEN : span_name_len;
    bpf_probe_read(span_name->buf, span_name_size, span_name_ptr);
}

static __always_inline long fill_partial_tracer_id_from_tracers_map(void *tracers_map, go_tracer_ptr tracer, tracer_id_t *tracer_id) {
    u64 tracers_count = 0;
    long res = 0;
    res = bpf_probe_read(&tracers_count, sizeof(tracers_count), tracers_map);
    if (res < 0)
    {
        return -1;
    }
    if (tracers_count == 0)
    {
        return -1;
    }
    unsigned char log_2_bucket_count;
    res = bpf_probe_read(&log_2_bucket_count, sizeof(log_2_bucket_count), tracers_map + 9);
    if (res < 0)
    {
        return -1;
    }
    u64 bucket_count = 1 << log_2_bucket_count;
    void *buckets_array;
    res = bpf_probe_read(&buckets_array, sizeof(buckets_array), (void*)(tracers_map + buckets_ptr_pos));
    if (res < 0)
    {
        return -1;
    }
    u32 map_id = 0;
    MAP_BUCKET_TYPE(go_tracer_id_partial_t, go_tracer_ptr) *map_bucket = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
    if (!map_bucket)
    {
        return -1;
    }

    for (u64 j = 0; j < MAX_BUCKETS; j++)
    {
        if (j >= bucket_count)
        {
            break;
        }
        res = bpf_probe_read(map_bucket, sizeof(MAP_BUCKET_TYPE(go_tracer_id_partial_t, go_tracer_ptr)), buckets_array + (j * sizeof(MAP_BUCKET_TYPE(go_tracer_id_partial_t, go_tracer_ptr))));
        if (res < 0)
        {
            continue;
        }
        for (u64 i = 0; i < 8; i++)
        {
            if (map_bucket->tophash[i] == 0)
            {
                continue;
            }
            if (map_bucket->values[i] == NULL)
            {
                continue;
            }
            if (map_bucket->values[i] != tracer)
            {
                continue;
            }
            get_go_string_from_user_ptr(&map_bucket->keys[i].version, tracer_id->version, MAX_TRACER_VERSION_LEN);
            return 0;
        }
    }
    return 0;
}

static __always_inline long fill_tracer_id_with_schema_from_tracers_map(void *tracers_map, go_tracer_ptr tracer, tracer_id_t *tracer_id) {
    u64 tracers_count = 0;
    long res = 0;
    res = bpf_probe_read(&tracers_count, sizeof(tracers_count), tracers_map);
    if (res < 0)
    {
        return -1;
    }
    if (tracers_count == 0)
    {
        return -1;
    }
    unsigned char log_2_bucket_count;
    res = bpf_probe_read(&log_2_bucket_count, sizeof(log_2_bucket_count), tracers_map + 9);
    if (res < 0)
    {
        return -1;
    }
    u64 bucket_count = 1 << log_2_bucket_count;
    void *buckets_array;
    res = bpf_probe_read(&buckets_array, sizeof(buckets_array), (void*)(tracers_map + buckets_ptr_pos));
    if (res < 0)
    {
        return -1;
    }
    u32 map_id = 0;
    MAP_BUCKET_TYPE(go_tracer_with_schema_t, go_tracer_ptr) *map_bucket = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
    if (!map_bucket)
    {
        return -1;
    }

    for (u64 j = 0; j < MAX_BUCKETS; j++)
    {
        if (j >= bucket_count)
        {
            break;
        }
        res = bpf_probe_read(map_bucket, sizeof(MAP_BUCKET_TYPE(go_tracer_with_schema_t, go_tracer_ptr)), buckets_array + (j * sizeof(MAP_BUCKET_TYPE(go_tracer_with_schema_t, go_tracer_ptr))));
        if (res < 0)
        {
            continue;
        }
        for (u64 i = 0; i < 8; i++)
        {
            if (map_bucket->tophash[i] == 0)
            {
                continue;
            }
            if (map_bucket->values[i] == NULL)
            {
                continue;
            }
            if (map_bucket->values[i] != tracer)
            {
                continue;
            }
            get_go_string_from_user_ptr(&map_bucket->keys[i].version, tracer_id->version, MAX_TRACER_VERSION_LEN);
            get_go_string_from_user_ptr(&map_bucket->keys[i].schema_url, tracer_id->schema_url, MAX_TRACER_SCHEMA_URL_LEN);
            return 0;
        }
    }
    return 0;
}

static __always_inline long fill_tracer_id_with_scope_attributes_from_tracers_map(void *tracers_map, go_tracer_ptr tracer, tracer_id_t *tracer_id) {
    u64 tracers_count = 0;
    long res = 0;
    res = bpf_probe_read(&tracers_count, sizeof(tracers_count), tracers_map);
    if (res < 0)
    {
        return -1;
    }
    if (tracers_count == 0)
    {
        return -1;
    }
    unsigned char log_2_bucket_count;
    res = bpf_probe_read(&log_2_bucket_count, sizeof(log_2_bucket_count), tracers_map + 9);
    if (res < 0)
    {
        return -1;
    }
    u64 bucket_count = 1 << log_2_bucket_count;
    void *buckets_array;
    res = bpf_probe_read(&buckets_array, sizeof(buckets_array), (void*)(tracers_map + buckets_ptr_pos));
    if (res < 0)
    {
        return -1;
    }
    u32 map_id = 0;
    MAP_BUCKET_TYPE(go_tracer_with_scope_attributes_t, go_tracer_ptr) *map_bucket = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
    if (!map_bucket)
    {
        return -1;
    }

    for (u64 j = 0; j < MAX_BUCKETS; j++)
    {
        if (j >= bucket_count)
        {
            break;
        }
        res = bpf_probe_read(map_bucket, sizeof(MAP_BUCKET_TYPE(go_tracer_with_scope_attributes_t, go_tracer_ptr)), buckets_array + (j * sizeof(MAP_BUCKET_TYPE(go_tracer_with_scope_attributes_t, go_tracer_ptr))));
        if (res < 0)
        {
            continue;
        }
        for (u64 i = 0; i < 8; i++)
        {
            if (map_bucket->tophash[i] == 0)
            {
                continue;
            }
            if (map_bucket->values[i] == NULL)
            {
                continue;
            }
            if (map_bucket->values[i] != tracer)
            {
                continue;
            }
            get_go_string_from_user_ptr(&map_bucket->keys[i].version, tracer_id->version, MAX_TRACER_VERSION_LEN);
            get_go_string_from_user_ptr(&map_bucket->keys[i].schema_url, tracer_id->schema_url, MAX_TRACER_SCHEMA_URL_LEN);
            return 0;
        }
    }
    return 0;
}

static __always_inline long fill_tracer_id(tracer_id_t *tracer_id, go_tracer_ptr tracer) {
    // Check if the tracer id is already cached
    tracer_id_t *cached_tracer_id = bpf_map_lookup_elem(&tracer_ptr_to_id_map, &tracer);
    if (cached_tracer_id != NULL) {
        *tracer_id = *cached_tracer_id;
        return 0;
    }

    if (!get_go_string_from_user_ptr((void*)(tracer + tracer_name_pos), tracer_id->name, MAX_TRACER_NAME_LEN)) {
        return -1;
    }

    long res = 0;
    void *tracer_provider = NULL;
    res = bpf_probe_read(&tracer_provider, sizeof(tracer_provider), (void*)(tracer + tracer_provider_pos));
    if (res < 0) {
        return res;
    }

    void *tracers_map = NULL;
    res = bpf_probe_read(&tracers_map, sizeof(tracers_map), (void*)(tracer_provider + tracer_provider_tracers_pos));
    if (res < 0) {
        return res;
    }

    if (tracer_id_contains_schemaURL) {
        // version of otel-go is 1.28.0 or higher
        if (tracer_id_contains_scope_attributes) {
            // version of otel-go is 1.32.0 or higher
            // we don't collect the scope attributes, but we need to take their presence into account,
            // when parsing the map bucket
            res = fill_tracer_id_with_scope_attributes_from_tracers_map(tracers_map, tracer, tracer_id);
        } else {
            res = fill_tracer_id_with_schema_from_tracers_map(tracers_map, tracer, tracer_id);
        }
    } else {
        res = fill_partial_tracer_id_from_tracers_map(tracers_map, tracer, tracer_id);
    }
    if (res < 0) {
        return res;
    }

    bpf_map_update_elem(&tracer_ptr_to_id_map, &tracer, tracer_id, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (t *tracer) newSpan(ctx context.Context, autoSpan *bool, name string, opts []trace.SpanStartOption) (context.Context, trace.Span) {
// https://github.com/open-telemetry/opentelemetry-go/blob/ac386f383cdfc14f546b4e55e8726a0a45e8a409/internal/global/trace.go#L161
SEC("uprobe/newSpan")
int uprobe_newStart(struct pt_regs *ctx) {
    if (wrote_flag) {
        // Already wrote flag value.
        return 0;
    }

    void *flag_ptr = get_argument(ctx, 4);
    if (flag_ptr == NULL) {
        bpf_printk("invalid flag_ptr: NULL");
        return -1;
    }

    bool true_value = true;
    long res = bpf_probe_write_user(flag_ptr, &true_value, sizeof(bool));
    if (res != 0) {
        bpf_printk("failed to write bool flag value: %ld", res);
        return -2;
    }

    wrote_flag = true;

    // Signal this uprobe should be unloaded.
    struct control_t ctrl = {1};
    return bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, (void *)(&ctrl), sizeof(struct control_t));
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
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    void *key = get_consistent_key(ctx, go_context.data);
    bpf_map_update_elem(&span_name_by_context, &key, &span_name, 0);

    // Get the tracer id
    u32 map_id = 0;
    tracer_id_t *tracer_id = bpf_map_lookup_elem(&tracer_id_storage_map, &map_id);
    if (tracer_id == NULL) {
        return 0;
    }
    __builtin_memset(tracer_id, 0, sizeof(tracer_id_t));

    long res = fill_tracer_id(tracer_id, tracer_ptr);
    if (res < 0) {
        return 0;
    }
    bpf_map_update_elem(&tracer_id_by_context, &key, tracer_id, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (t *tracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
// https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/internal/global/trace.go#L149
SEC("uprobe/Start")
int uprobe_Start_Returns(struct pt_regs *ctx) {
    // Get the span name passed to the Start function
    struct go_iface go_context = {0};
    // In return probe, the context is the first return value
    get_Go_context(ctx, 1, 0, true, &go_context);
    void *key = get_consistent_key(ctx, go_context.data);
    struct span_name_t *span_name = bpf_map_lookup_elem(&span_name_by_context, &key); 
    if (span_name == NULL) {
        return 0;
    }

    tracer_id_t *tracer_id = bpf_map_lookup_elem(&tracer_id_by_context, &key);
    if (tracer_id == NULL) {
        goto done_without_tracer_id;
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
    otel_span->span_name = *span_name;
    otel_span->tracer_id = *tracer_id;

    // Get the ** returned ** Span (concrete type of the interfaces)
    void *span_ptr_val = get_argument(ctx, 4);

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &otel_span->psc,
        .sc = &otel_span->sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    bpf_map_update_elem(&active_spans_by_span_ptr, &span_ptr_val, otel_span, 0);
    start_tracking_span(go_context.data, &otel_span->sc);

done:
    bpf_map_delete_elem(&tracer_id_by_context, &key);
done_without_tracer_id:
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
