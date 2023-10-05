#include "arguments.h"
#include "span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_PATH_SIZE 100
#define MAX_METHOD_SIZE 10
#define W3C_KEY_LENGTH 11
#define W3C_VAL_LENGTH 55
#define MAX_CONCURRENT 50

struct http_request_t {
    BASE_SPAN_PROPERTIES
    char method[MAX_METHOD_SIZE];
    char path[MAX_PATH_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct http_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} http_events SEC(".maps");

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
volatile const u64 buckets_ptr_pos;

static __always_inline long inject_header(void* headers_ptr, struct span_context* propagated_ctx) {
    // Read the key-value count - this field must be the first one in the hmap struct as documented in src/runtime/map.go
    u64 curr_keyvalue_count = 0;
    long res = bpf_probe_read_user(&curr_keyvalue_count, sizeof(curr_keyvalue_count), headers_ptr);

    if (res < 0) {
        bpf_printk("Couldn't read map key-value count from user");
        return -1;
    }

    if (curr_keyvalue_count >= 8) {
        bpf_printk("Map size is bigger than 8, skipping context propagation");
        return -1;
    }

    // Get pointer to temp bucket struct we store in a map (avoiding large local variable on the stack)
    // Performing read-modify-write on the bucket
    u32 map_id = 0;
    struct map_bucket *bucket_map_value = bpf_map_lookup_elem(&golang_mapbucket_storage_map, &map_id);
    if (!bucket_map_value) {
        return -1;
    }

    void *buckets_ptr_ptr = (void*) (headers_ptr + buckets_ptr_pos);
    void *bucket_ptr = 0; // The actual pointer to the buckets

    if (curr_keyvalue_count == 0) {
        // No key-value pairs in the Go map, need to "allocate" memory for the user
        bucket_ptr = write_target_data(bucket_map_value, sizeof(struct map_bucket));
        if (bucket_ptr == NULL) {
            bpf_printk("inject_header: Failed to write bucket to user");
            return -1;
        }
        // Update the buckets pointer in the hmap struct to point to newly allocated bucket
        res = bpf_probe_write_user(buckets_ptr_ptr, &bucket_ptr, sizeof(bucket_ptr));
        if (res < 0) {
            bpf_printk("Failed to update the map bucket pointer for the user");
            return -1;
        }
    } else {
        // There is at least 1 key-value pair, hence the bucket pointer from the user is valid
        // Read the bucket pointer
        res = bpf_probe_read_user(&bucket_ptr, sizeof(bucket_ptr), buckets_ptr_ptr);
        // Read the user's bucket to the eBPF map entry
        bpf_probe_read_user(bucket_map_value, sizeof(struct map_bucket), bucket_ptr);
    }

    u8 bucket_index = curr_keyvalue_count & 0x7;

    char traceparent_tophash = 0xee;
    bucket_map_value->tophash[bucket_index] = traceparent_tophash;

    // Prepare the key string for the user
    char key[W3C_KEY_LENGTH] = "traceparent";
    void *ptr = write_target_data(key, W3C_KEY_LENGTH);
    if (ptr == NULL) {
        bpf_printk("inject_header: Failed to write key to user");
        return -1;
    }
    bucket_map_value->keys[bucket_index] = (struct go_string) {.len = W3C_KEY_LENGTH, .str = ptr};

    // Prepare the value string slice
    // First the value string which constains the span context
    char val[W3C_VAL_LENGTH];
    span_context_to_w3c_string(propagated_ctx, val);
    ptr = write_target_data(val, sizeof(val));
    if(ptr == NULL) {
        bpf_printk("inject_header: Failed to write value to user");
        return -1;
    }

    // The go string pointing to the above val
    struct go_string header_value = {.len = W3C_VAL_LENGTH, .str = ptr};
    ptr = write_target_data((void*)&header_value, sizeof(header_value));
    if(ptr == NULL) {
        bpf_printk("inject_header: Failed to write go_string to user");
        return -1;
    }

    // Last, go_slice pointing to the above go_string
    bucket_map_value->values[bucket_index] = (struct go_slice) {.array = ptr, .cap = 1, .len = 1};

    // Update the map header count field
    curr_keyvalue_count += 1;
    res = bpf_probe_write_user(headers_ptr, &curr_keyvalue_count, sizeof(curr_keyvalue_count));
    if (res < 0) {
        bpf_printk("Failed to update key-value count in map header");
        return -1;
    }

    // Update the bucket
    res = bpf_probe_write_user(bucket_ptr, bucket_map_value, sizeof(struct map_bucket));
    if (res < 0) {
        bpf_printk("Failed to update bucket content");
        return -1;
    }

    bpf_memset((unsigned char *)bucket_map_value, sizeof(struct map_bucket), 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func net/http/client.Do(req *Request)
SEC("uprobe/HttpClient_Do")
int uprobe_HttpClient_Do(struct pt_regs *ctx) {
    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_ns();

    u64 request_pos = 2;
    void *req_ptr = get_argument(ctx, request_pos);

    // Get parent if exists
    void *context_ptr = (void *)(req_ptr+ctx_ptr_pos);
    void *context_ptr_val = 0;
    bpf_probe_read(&context_ptr_val, sizeof(context_ptr_val), context_ptr);
    struct span_context *parent_span_ctx = get_parent_span_context(context_ptr_val);
    if (parent_span_ctx != NULL) {
        bpf_probe_read(&httpReq.psc, sizeof(httpReq.psc), parent_span_ctx);
        copy_byte_arrays(httpReq.psc.TraceID, httpReq.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(httpReq.sc.SpanID, SPAN_ID_SIZE);
    } else {
        httpReq.sc = generate_span_context();
        bpf_memset(httpReq.psc.SpanID, SPAN_ID_SIZE, 0);
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

    // Get key
    void *key = get_consistent_key(ctx, context_ptr);

    // Write event
    bpf_map_update_elem(&http_events, &key, &httpReq, 0);
    start_tracking_span(context_ptr_val, &httpReq.sc);
    return 0;
}

// This instrumentation attaches uretprobe to the following function:
// func net/http/client.Do(req *Request)
UPROBE_RETURN(HttpClient_Do, struct http_request_t, 2, ctx_ptr_pos, http_events, events, true)