#include "arguments.h"
#include "span_context.h"
#include "go_context.h"
#include "go_types.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_PATH_SIZE 100
#define MAX_METHOD_SIZE 10
#define W3C_KEY_LENGTH 11
#define W3C_VAL_LENGTH 55
#define MAX_CONCURRENT 50

struct http_request_t {
    u64 start_time;
    u64 end_time;
    char method[MAX_METHOD_SIZE];
    char path[MAX_PATH_SIZE];
    struct span_context sc;
    struct span_context psc;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct http_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} context_to_http_events SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(struct map_bucket));
	__uint(max_entries, 1);
} golang_mapbucket_storage_map SEC(".maps");


struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 headers_ptr_pos;
volatile const u64 ctx_ptr_pos;

static __always_inline long inject_header(void* headers_ptr, struct span_context* propagated_ctx) {

    // Read headers map count
    u64 map_keyvalue_count = 0;
    bpf_probe_read(&map_keyvalue_count, sizeof(map_keyvalue_count), headers_ptr);

    // Currently only maps with less than 8 keys are supported for injection
    if (map_keyvalue_count >= 8) {
        return 0;
    }
    long res;
    if (map_keyvalue_count == 0) {
        u32 map_id = 0;
        struct map_bucket *map_value = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
        if (!map_value) {
            return -1;
        }
        void *bucket_ptr = write_target_data(map_value, sizeof(struct map_bucket));
        res = bpf_probe_write_user(headers_ptr + 16, &bucket_ptr, sizeof(bucket_ptr));

        if(res < 0) {
            return -1;
        }

    }

    void *map_keyvalues_ptr = NULL;

    bpf_probe_read(&map_keyvalues_ptr, sizeof(map_keyvalues_ptr), headers_ptr + 16);

    void *injected_key_ptr = map_keyvalues_ptr + 8 + (16 * map_keyvalue_count);

    char traceparent_tophash = 0xee;
    void *tophashes_ptr = map_keyvalues_ptr +  map_keyvalue_count;
    res = bpf_probe_write_user(tophashes_ptr, &traceparent_tophash, 1);

    if(res < 0) {
        return -1;
    }

    char key[W3C_KEY_LENGTH] = "traceparent";
    void *ptr = write_target_data(key, W3C_KEY_LENGTH);

    res = bpf_probe_write_user(injected_key_ptr, &ptr, sizeof(ptr));

    if(res < 0) {
        return -1;
    }

    u64 header_key_length = W3C_KEY_LENGTH;
    res = bpf_probe_write_user(injected_key_ptr + 8, &header_key_length, sizeof(header_key_length));

    if(res < 0) {
        return -1;
    }

    void *injected_value_ptr = injected_key_ptr + (16 * (8 - map_keyvalue_count)) + 24 * map_keyvalue_count;



    char val[W3C_VAL_LENGTH];

    span_context_to_w3c_string(propagated_ctx, val);

    ptr = write_target_data(val, sizeof(val));
    struct go_string header_value = {};
    header_value.str = ptr;
    header_value.len = W3C_VAL_LENGTH;

    ptr = write_target_data((void*)&header_value, sizeof(header_value));

    if(ptr == NULL) {
        return -1;
    }

    struct go_slice values_slice = {};
    values_slice.array = ptr;
    values_slice.len = 1;
    values_slice.cap = 1;

    res = bpf_probe_write_user(injected_value_ptr, &values_slice, sizeof(values_slice));

    if(res < 0) {
        return -1;
    }

    map_keyvalue_count += 1;
    res = bpf_probe_write_user(headers_ptr, &map_keyvalue_count, sizeof(map_keyvalue_count));

    if(res < 0) {
        return -1;
    }

    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func net/http/client.Do(req *Request)
SEC("uprobe/HttpClient")
int uprobe_HttpClient_Do(struct pt_regs *ctx) {

    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_ns();


    u64 request_pos = 2;
    void *req_ptr = get_argument(ctx, request_pos);

    // Get Request.ctx
    void *goroutine = get_goroutine_address(ctx, ctx_ptr_pos);
    // Get parent if exists
    struct span_context *span_ctx = bpf_map_lookup_elem(&spans_in_progress, &goroutine);
    if (span_ctx != NULL) {
        bpf_probe_read(&httpReq.psc, sizeof(httpReq.psc), span_ctx);
        copy_byte_arrays(httpReq.psc.TraceID, httpReq.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(httpReq.sc.SpanID, SPAN_ID_SIZE);
    } else {
        httpReq.sc = generate_span_context();
    }

    void *method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(req_ptr+method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(req_ptr+(method_ptr_pos+8)));
    u64 method_size = sizeof(httpReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&httpReq.method, method_size, method_ptr);

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr+url_ptr_pos));
    void *path_ptr = 0;
    bpf_probe_read(&path_ptr, sizeof(path_ptr), (void *)(url_ptr+path_ptr_pos));

    u64 path_len = 0;
    bpf_probe_read(&path_len, sizeof(path_len), (void *)(url_ptr+(path_ptr_pos+8)));
    u64 path_size = sizeof(httpReq.path);
    path_size = path_size < path_len ? path_size : path_len;
    bpf_probe_read(&httpReq.path, path_size, path_ptr);

    // get headers from Request
    void *headers_ptr = 0;
    bpf_probe_read(&headers_ptr, sizeof(headers_ptr), (void *)(req_ptr+headers_ptr_pos));
    u64 map_keyvalue_count = 0;
    bpf_probe_read(&map_keyvalue_count, sizeof(map_keyvalue_count), headers_ptr);
    long res = inject_header(headers_ptr, &httpReq.sc);
    if (res < 0) {
        bpf_printk("uprobe_HttpClient_Do: Failed to inject header");
    }

    bpf_map_update_elem(&context_to_http_events, &goroutine, &httpReq, 0);
    bpf_map_update_elem(&spans_in_progress, &goroutine, &httpReq.sc, 0);

    return 0;
}

// This instrumentation attaches uretprobe to the following function:
// func net/http/client.Do(req *Request)
SEC("uprobe/HttpClient")
int uprobe_HttpClient_Do_Returns(struct pt_regs *ctx) {
    u64 request_pos = 2;
    void *req_ptr = get_argument_by_stack(ctx, request_pos);
    void *goroutine = get_goroutine_address(ctx, ctx_ptr_pos);
    void *httpReq_ptr = bpf_map_lookup_elem(&context_to_http_events, &goroutine);
    struct http_request_t httpReq = {};
    bpf_probe_read(&httpReq, sizeof(httpReq), httpReq_ptr);
    httpReq.end_time = bpf_ktime_get_ns();
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &httpReq, sizeof(httpReq));

    bpf_map_delete_elem(&context_to_http_events, &goroutine);
    bpf_map_delete_elem(&spans_in_progress, &goroutine);

    return 0;
}