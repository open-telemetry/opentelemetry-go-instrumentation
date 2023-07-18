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

#ifndef SPAN_CONTEXT_H
#define SPAN_CONTEXT_H

#include "utils.h"
#include "go_types.h"

#define SPAN_CONTEXT_STRING_SIZE 55
#define MAX_CONCURRENT_SPANS 100
#define MAX_BUCKETS 8
#define W3C_KEY_LENGTH 11
#define W3C_VAL_LENGTH 55

struct span_context
{
    unsigned char TraceID[TRACE_ID_SIZE];
    unsigned char SpanID[SPAN_ID_SIZE];
};

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct map_bucket));
    __uint(max_entries, 1);
} golang_mapbucket_storage_map SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct span_context));
    __uint(max_entries, 1);
} parent_span_context_storage_map SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct span_context);
    __uint(max_entries, MAX_CONCURRENT_SPANS);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} spans_in_progress SEC(".maps");

static __always_inline struct span_context generate_span_context()
{
    struct span_context context = {};
    generate_random_bytes(context.TraceID, TRACE_ID_SIZE);
    generate_random_bytes(context.SpanID, SPAN_ID_SIZE);
    return context;
}

static __always_inline void span_context_to_w3c_string(struct span_context *ctx, char *buff)
{
    // W3C format: version (2 chars) - trace id (32 chars) - span id (16 chars) - sampled (2 chars)
    char *out = buff;

    // Write version
    *out++ = '0';
    *out++ = '0';
    *out++ = '-';

    // Write trace id
    bytes_to_hex_string(ctx->TraceID, TRACE_ID_SIZE, out);
    out += TRACE_ID_STRING_SIZE;
    *out++ = '-';

    // Write span id
    bytes_to_hex_string(ctx->SpanID, SPAN_ID_SIZE, out);
    out += SPAN_ID_STRING_SIZE;
    *out++ = '-';

    // Write sampled
    *out++ = '0';
    *out = '1';
}

static __always_inline void w3c_string_to_span_context(char *str, struct span_context *ctx)
{
    u32 trace_id_start_pos = 3;
    u32 span_id_start_pod = 36;
    hex_string_to_bytes(str + trace_id_start_pos, TRACE_ID_STRING_SIZE, ctx->TraceID);
    hex_string_to_bytes(str + span_id_start_pod, SPAN_ID_STRING_SIZE, ctx->SpanID);
}

static __always_inline struct span_context *extract_context_from_req_headers(void *headers_ptr_ptr)
{
    void *headers_ptr;
    long res;
    res = bpf_probe_read(&headers_ptr, sizeof(headers_ptr), headers_ptr_ptr);
    if (res < 0)
    {
        return NULL;
    }
    u64 headers_count = 0;
    res = bpf_probe_read(&headers_count, sizeof(headers_count), headers_ptr);
    if (res < 0)
    {
        return NULL;
    }
    if (headers_count == 0)
    {
        return NULL;
    }
    unsigned char log_2_bucket_count;
    res = bpf_probe_read(&log_2_bucket_count, sizeof(log_2_bucket_count), headers_ptr + 9);
    if (res < 0)
    {
        return NULL;
    }
    u64 bucket_count = 1 << log_2_bucket_count;
    void *header_buckets;
    res = bpf_probe_read(&header_buckets, sizeof(header_buckets), headers_ptr + 16);
    if (res < 0)
    {
        return NULL;
    }
    u32 map_id = 0;
    struct map_bucket *map_value = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
    if (!map_value)
    {
        return NULL;
    }

    for (u64 j = 0; j < MAX_BUCKETS; j++)
    {
        if (j >= bucket_count)
        {
            break;
        }
        res = bpf_probe_read(map_value, sizeof(struct map_bucket), header_buckets + (j * sizeof(struct map_bucket)));
        if (res < 0)
        {
            continue;
        }
        for (u64 i = 0; i < 8; i++)
        {
            if (map_value->tophash[i] == 0)
            {
                continue;
            }
            if (map_value->keys[i].len != W3C_KEY_LENGTH)
            {
                continue;
            }
            char current_header_key[W3C_KEY_LENGTH];
            bpf_probe_read(current_header_key, sizeof(current_header_key), map_value->keys[i].str);
            if (!bpf_memcmp(current_header_key, "traceparent", W3C_KEY_LENGTH) && !bpf_memcmp(current_header_key, "Traceparent", W3C_KEY_LENGTH))
            {
                continue;
            }
            void *traceparent_header_value_ptr = map_value->values[i].array;
            struct go_string traceparent_header_value_go_str;
            res = bpf_probe_read(&traceparent_header_value_go_str, sizeof(traceparent_header_value_go_str), traceparent_header_value_ptr);
            if (res < 0)
            {
                return NULL;
            }
            if (traceparent_header_value_go_str.len != W3C_VAL_LENGTH)
            {
                continue;
            }
            char traceparent_header_value[W3C_VAL_LENGTH];
            res = bpf_probe_read(&traceparent_header_value, sizeof(traceparent_header_value), traceparent_header_value_go_str.str);
            if (res < 0)
            {
                return NULL;
            }
            struct span_context *parent_span_context = bpf_map_lookup_elem(&parent_span_context_storage_map, &map_id);
            if (!parent_span_context)
            {
                return NULL;
            }
            w3c_string_to_span_context(traceparent_header_value, parent_span_context);
            return parent_span_context;
        }
    }
    return NULL;
}

#endif