// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "trace/span_output.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_HOSTNAME_SIZE 128
#define MAX_PROTO_SIZE 8
#define MAX_PATH_SIZE 128
#define MAX_SCHEME_SIZE 8
#define MAX_OPAQUE_SIZE 8
#define MAX_RAWPATH_SIZE 8
#define MAX_RAWQUERY_SIZE 128
#define MAX_FRAGMENT_SIZE 56
#define MAX_RAWFRAGMENT_SIZE 56
#define MAX_USERNAME_SIZE 8
#define MAX_METHOD_SIZE 16
#define MAX_CONCURRENT 56

struct http_request_t {
    BASE_SPAN_PROPERTIES
    char host[MAX_HOSTNAME_SIZE];
    char proto[MAX_PROTO_SIZE];
    u64 status_code;
    char method[MAX_METHOD_SIZE];
    char path[MAX_PATH_SIZE];
    char scheme[MAX_SCHEME_SIZE];
    char opaque[MAX_OPAQUE_SIZE];
    char raw_path[MAX_RAWPATH_SIZE];
    char username[MAX_USERNAME_SIZE];
    char raw_query[MAX_RAWQUERY_SIZE];
    char fragment[MAX_FRAGMENT_SIZE];
    char raw_fragment[MAX_RAWFRAGMENT_SIZE];
    u8 force_query;
    u8 omit_host;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct http_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} http_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct http_request_t));
    __uint(max_entries, 1);
} http_client_uprobe_storage_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, void*); // the headers ptr
	__type(value, void*); // request key, goroutine or context ptr
	__uint(max_entries, MAX_CONCURRENT);
} http_headers SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 headers_ptr_pos;
volatile const u64 ctx_ptr_pos;
volatile const u64 status_code_pos;
volatile const u64 request_host_pos;
volatile const u64 request_proto_pos;
volatile const u64 scheme_pos;
volatile const u64 opaque_pos;
volatile const u64 user_ptr_pos;
volatile const u64 raw_path_pos;
volatile const u64 omit_host_pos;
volatile const u64 force_query_pos;
volatile const u64 raw_query_pos;
volatile const u64 fragment_pos;
volatile const u64 raw_fragment_pos;
volatile const u64 username_pos;
volatile const u64 io_writer_buf_ptr_pos;
volatile const u64 io_writer_n_pos;
volatile const u64 url_host_pos;

// This instrumentation attaches uprobe to the following function:
// func net/http/transport.roundTrip(req *Request) (*Response, error)
SEC("uprobe/Transport_roundTrip")
int uprobe_Transport_roundTrip(struct pt_regs *ctx) {
    u64 request_pos = 2;
    void *req_ptr = get_argument(ctx, request_pos);

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, ctx_ptr_pos, false, &go_context);

    void *key = (void *)GOROUTINE(ctx);
    void *httpReq_ptr = bpf_map_lookup_elem(&http_events, &key);
    if (httpReq_ptr != NULL)
    {
        bpf_printk("uprobe/Transport_RoundTrip already tracked with the current context");
        return 0;
    }

    u32 map_id = 0;
    struct http_request_t *httpReq = bpf_map_lookup_elem(&http_client_uprobe_storage_map, &map_id);
    if (httpReq == NULL)
    {
        bpf_printk("uprobe/Transport_roundTrip: httpReq is NULL");
        return 0;
    }

    __builtin_memset(httpReq, 0, sizeof(struct http_request_t));
    httpReq->start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &httpReq->psc,
        .sc = &httpReq->sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    if (!get_go_string_from_user_ptr((void *)(req_ptr+method_ptr_pos), httpReq->method, sizeof(httpReq->method))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get method from request");
        return 0;
    }

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr+url_ptr_pos));
    if (!get_go_string_from_user_ptr((void *)(url_ptr+path_ptr_pos), httpReq->path, sizeof(httpReq->path))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get path from Request.URL");
    }

    // get scheme from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+scheme_pos), httpReq->scheme, sizeof(httpReq->scheme))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get scheme from Request.URL");
    }

    // get opaque from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+opaque_pos), httpReq->opaque, sizeof(httpReq->opaque))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get opaque from Request.URL");
    }

    // get RawPath from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+raw_path_pos), httpReq->raw_path, sizeof(httpReq->raw_path))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get RawPath from Request.URL");
    }

    // get username from Request.URL.User
    void *user_ptr = 0;
    bpf_probe_read(&user_ptr, sizeof(user_ptr), (void *)(url_ptr+user_ptr_pos));
    if (!get_go_string_from_user_ptr((void *)(user_ptr+username_pos), httpReq->username, sizeof(httpReq->username))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get RawQuery from Request.URL");
    }

    // get RawQuery from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+raw_query_pos), httpReq->raw_query, sizeof(httpReq->raw_query))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get RawQuery from Request.URL");
    }

    // get Fragment from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+fragment_pos), httpReq->fragment, sizeof(httpReq->fragment))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get Fragment from Request.URL");
    }

    // get RawFragment from Request.URL
    if (!get_go_string_from_user_ptr((void *)(url_ptr+raw_fragment_pos), httpReq->raw_fragment, sizeof(httpReq->raw_fragment))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get RawFragment from Request.URL");
    }

    // get ForceQuery from Request.URL
    bpf_probe_read(&httpReq->force_query, sizeof(httpReq->force_query), (void *)(url_ptr+force_query_pos));

    // get OmitHost from Request.URL
    bpf_probe_read(&httpReq->omit_host, sizeof(httpReq->omit_host), (void *)(url_ptr+omit_host_pos));

    // get host from Request
    if (!get_go_string_from_user_ptr((void *)(req_ptr+request_host_pos), httpReq->host, sizeof(httpReq->host))) {
        // If host is not present in Request, get it from URL
        if (!get_go_string_from_user_ptr((void *)(url_ptr+url_host_pos), httpReq->host, sizeof(httpReq->host))) {
            bpf_printk("uprobe_Transport_roundTrip: Failed to get host from Request and URL");
        }
    }

    // get proto from Request
    if (!get_go_string_from_user_ptr((void *)(req_ptr+request_proto_pos), httpReq->proto, sizeof(httpReq->proto))) {
        bpf_printk("uprobe_Transport_roundTrip: Failed to get proto from Request");
    }

    // get headers from Request
    void *headers_ptr = 0;
    bpf_probe_read(&headers_ptr, sizeof(headers_ptr), (void *)(req_ptr+headers_ptr_pos));
    if (headers_ptr) {
        bpf_map_update_elem(&http_headers, &headers_ptr, &key, 0);
    }

    // Write event
    bpf_map_update_elem(&http_events, &key, httpReq, 0);
    return 0;
}

// This instrumentation attaches uretprobe to the following function:
// func net/http/transport.roundTrip(req *Request) (*Response, error)
SEC("uprobe/Transport_roundTrip")
int uprobe_Transport_roundTrip_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();
    void *key = (void *)GOROUTINE(ctx);

    struct http_request_t *http_req_span = bpf_map_lookup_elem(&http_events, &key);
    if (http_req_span == NULL) {
        bpf_printk("probe_Transport_roundTrip_Returns: entry_state is NULL");
        return 0;
    }

    // Getting the returned response
    void *resp_ptr = get_argument(ctx, 1);
    // Get status code from response
    bpf_probe_read(&http_req_span->status_code, sizeof(http_req_span->status_code), (void *)(resp_ptr + status_code_pos));

    http_req_span->end_time = end_time;

    output_span_event(ctx, http_req_span, sizeof(*http_req_span), &http_req_span->sc);

    bpf_map_delete_elem(&http_events, &key);
    return 0;
}

#ifndef NO_HEADER_PROPAGATION
// This instrumentation attaches uprobe to the following function:
// func (h Header) net/http.Header.writeSubset(w io.Writer, exclude map[string]bool, trace *httptrace.ClientTrace) error
SEC("uprobe/header_writeSubset")
int uprobe_writeSubset(struct pt_regs *ctx) {
    u64 headers_pos = 1;
    void *headers_ptr = get_argument(ctx, headers_pos);

    u64 io_writer_pos = 3;
    void *io_writer_ptr = get_argument(ctx, io_writer_pos);

    void **key_ptr = bpf_map_lookup_elem(&http_headers, &headers_ptr);
    if (key_ptr) {
        void *key = *key_ptr;

        struct http_request_t *http_req_span = bpf_map_lookup_elem(&http_events, &key);
        if (http_req_span) {
            char tp[W3C_VAL_LENGTH];
            span_context_to_w3c_string(&http_req_span->sc, tp);

            void *buf_ptr = 0;
            bpf_probe_read(&buf_ptr, sizeof(buf_ptr), (void *)(io_writer_ptr + io_writer_buf_ptr_pos)); // grab buf ptr
            if (!buf_ptr) {
                bpf_printk("uprobe_writeSubset: Failed to get buf from io writer");
                goto done;
            }

            s64 size = 0;
            if (bpf_probe_read(&size, sizeof(s64), (void *)(io_writer_ptr + io_writer_buf_ptr_pos + offsetof(struct go_slice, cap)))) { // grab capacity
                bpf_printk("uprobe_writeSubset: Failed to get size from io writer");
                goto done;
            }

            s64 len = 0;
            if (bpf_probe_read(&len, sizeof(s64), (void *)(io_writer_ptr + io_writer_n_pos))) { // grab len
                bpf_printk("uprobe_writeSubset: Failed to get len from io writer");
                goto done;
            }

            if (len < (size - W3C_VAL_LENGTH - W3C_KEY_LENGTH - 4)) { // 4 = strlen(":_") + strlen("\r\n")
                char tp_str[W3C_KEY_LENGTH + 2 + W3C_VAL_LENGTH + 2] = "Traceparent: ";
                char end[2] = "\r\n";
                __builtin_memcpy(&tp_str[W3C_KEY_LENGTH + 2], tp, sizeof(tp));
                __builtin_memcpy(&tp_str[W3C_KEY_LENGTH + 2 + W3C_VAL_LENGTH], end, sizeof(end));
                if (bpf_probe_write_user(buf_ptr + (len & 0x0ffff), tp_str, sizeof(tp_str))) {
                    bpf_printk("uprobe_writeSubset: Failed to write trace parent key in buffer");
                    goto done;
                }
                len += W3C_KEY_LENGTH + 2 + W3C_VAL_LENGTH + 2;
                if (bpf_probe_write_user((void *)(io_writer_ptr + io_writer_n_pos), &len, sizeof(len))) {
                    bpf_printk("uprobe_writeSubset: Failed to change io writer n");
                    goto done;
                }
            }
        }
    }

done:
    bpf_map_delete_elem(&http_headers, &headers_ptr);
    return 0;
}
#else
// Not used at all, empty stub needed to ensure both versions of the bpf program are
// able to compile with bpf2go. The userspace code will avoid loading the probe if
// context propagation is not enabled.
SEC("uprobe/header_writeSubset")
int uprobe_writeSubset(struct pt_regs *ctx) {
    return 0;
}
#endif
