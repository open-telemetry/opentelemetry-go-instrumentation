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

char __license[] SEC("license") = "Dual MIT/GPL";

#define PATH_MAX_LEN 128
#define MAX_BUCKETS 8
#define METHOD_MAX_LEN 8
#define MAX_CONCURRENT 50
#define REMOTE_ADDR_MAX_LEN 256
#define HOST_MAX_LEN 256
#define PROTO_MAX_LEN 8

struct http_server_span_t
{
    BASE_SPAN_PROPERTIES
    u64 status_code;
    char method[METHOD_MAX_LEN];
    char path[PATH_MAX_LEN];
    char remote_addr[REMOTE_ADDR_MAX_LEN];
    char host[HOST_MAX_LEN];
    char proto[PROTO_MAX_LEN];
};

struct uprobe_data_t
{
    struct http_server_span_t span;
    // bpf2go doesn't support pointers fields
    // saving the response pointer in the entry probe
    // and using it in the return probe
    u64 resp_ptr;
};

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct uprobe_data_t);
    __uint(max_entries, MAX_CONCURRENT);
} http_server_uprobes SEC(".maps");

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
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct uprobe_data_t));
    __uint(max_entries, 1);
} http_server_uprobe_storage_map SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 ctx_ptr_pos;
volatile const u64 headers_ptr_pos;
volatile const u64 buckets_ptr_pos;
volatile const u64 req_ptr_pos;
volatile const u64 status_code_pos;
volatile const u64 remote_addr_pos;
volatile const u64 host_pos;
volatile const u64 proto_pos;

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
    res = bpf_probe_read(&header_buckets, sizeof(header_buckets), (void*)(headers_ptr + buckets_ptr_pos));
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

// This instrumentation attaches uprobe to the following function:
// func (sh serverHandler) ServeHTTP(rw ResponseWriter, req *Request)
SEC("uprobe/HandlerFunc_ServeHTTP")
int uprobe_HandlerFunc_ServeHTTP(struct pt_regs *ctx)
{
    void *req_ctx_ptr = get_Go_context(ctx, 4, ctx_ptr_pos, false);
    void *key = get_consistent_key(ctx, req_ctx_ptr);
    void *httpReq_ptr = bpf_map_lookup_elem(&http_server_uprobes, &key);
    if (httpReq_ptr != NULL)
    {
        bpf_printk("uprobe/HandlerFunc_ServeHTTP already tracked with the current request");
        return 0;
    }

    u32 map_id = 0;
    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&http_server_uprobe_storage_map, &map_id);
    if (uprobe_data == NULL)
    {
        bpf_printk("uprobe/HandlerFunc_ServeHTTP: http_server_span is NULL");
        return 0;
    }

    __builtin_memset(uprobe_data, 0, sizeof(struct uprobe_data_t));

    // Save response writer
    void *resp_impl = get_argument(ctx, 3);
    uprobe_data->resp_ptr = (u64)resp_impl;

    struct http_server_span_t *http_server_span = &uprobe_data->span;
    http_server_span->start_time = bpf_ktime_get_ns();

    // Propagate context
    void *req_ptr = get_argument(ctx, 4);
    struct span_context *parent_ctx = extract_context_from_req_headers((void*)(req_ptr + headers_ptr_pos));
    if (parent_ctx != NULL)
    {
        // found parent context in http headers
        http_server_span->psc = *parent_ctx;
        copy_byte_arrays(http_server_span->psc.TraceID, http_server_span->sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(http_server_span->sc.SpanID, SPAN_ID_SIZE);
    }
    else
    {
        http_server_span->sc = generate_span_context();
    }

    if (req_ctx_ptr == NULL)
    {
        bpf_printk("uprobe/HandlerFunc_ServeHTTP: req_ctx_ptr is NULL");
        return 0;
    }

    bpf_map_update_elem(&http_server_uprobes, &key, uprobe_data, 0);
    start_tracking_span(req_ctx_ptr, &http_server_span->sc);
    return 0;
}

void read_go_string(void *base, int offset, char *output, int maxLen, const char *errorMsg) {
    void *ptr = (void *)(base + offset);
    if (!get_go_string_from_user_ptr(ptr, output, maxLen)) {
        bpf_printk("Failed to get %s", errorMsg);
    }
    
}

// This instrumentation attaches uprobe to the following function:
// func (sh serverHandler) ServeHTTP(rw ResponseWriter, req *Request)
SEC("uprobe/HandlerFunc_ServeHTTP")
int uprobe_HandlerFunc_ServeHTTP_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();
    void *req_ctx_ptr = get_Go_context(ctx, 4, ctx_ptr_pos, false);
    void *key = get_consistent_key(ctx, req_ctx_ptr);

    struct uprobe_data_t *uprobe_data = bpf_map_lookup_elem(&http_server_uprobes, &key);
    if (uprobe_data == NULL) {
        bpf_printk("uprobe/HandlerFunc_ServeHTTP_Returns: entry_state is NULL");
        return 0;
    }
    bpf_map_delete_elem(&http_server_uprobes, &key);

    struct http_server_span_t *http_server_span = &uprobe_data->span;
    void *resp_ptr = (void *)uprobe_data->resp_ptr;
    void *req_ptr = NULL;
    bpf_probe_read(&req_ptr, sizeof(req_ptr), (void *)(resp_ptr + req_ptr_pos));

    http_server_span->end_time = end_time;

    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr + url_ptr_pos));
    // Collect fields from response
    read_go_string(req_ptr, method_ptr_pos, http_server_span->method, sizeof(http_server_span->method), "method from request");
    read_go_string(url_ptr, path_ptr_pos, http_server_span->path, sizeof(http_server_span->path), "path from Request.URL");
    read_go_string(req_ptr, remote_addr_pos, http_server_span->remote_addr, sizeof(http_server_span->remote_addr), "remote addr from Request.RemoteAddr");
    read_go_string(req_ptr, host_pos, http_server_span->host, sizeof(http_server_span->host), "host from Request.Host");
    read_go_string(req_ptr, proto_pos, http_server_span->proto, sizeof(http_server_span->proto), "proto from Request.Proto");

    // status code
    bpf_probe_read(&http_server_span->status_code, sizeof(http_server_span->status_code), (void *)(resp_ptr + status_code_pos));

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, http_server_span, sizeof(*http_server_span));
    stop_tracking_span(&http_server_span->sc, &http_server_span->psc);
    return 0;
}