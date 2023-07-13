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

struct sql_request_t {
    u64 start_time;
    u64 end_time;
    char method[MAX_METHOD_SIZE];
    char path[MAX_PATH_SIZE];
    struct span_context sc;
    struct span_context psc;
};

// struct {
// 	__uint(type, BPF_MAP_TYPE_HASH);
// 	__type(key, void*);
// 	__type(value, struct http_request_t);
// 	__uint(max_entries, MAX_CONCURRENT);
// } context_to_http_events SEC(".maps");

// struct {
// 	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
// 	__uint(key_size, sizeof(u32));
// 	__uint(value_size, sizeof(struct map_bucket));
// 	__uint(max_entries, 1);
// } golang_mapbucket_storage_map SEC(".maps");


// struct {
// 	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
// } events SEC(".maps");

// Injected in init
// volatile const u64 method_ptr_pos;
// volatile const u64 url_ptr_pos;
// volatile const u64 path_ptr_pos;
// volatile const u64 headers_ptr_pos;
// volatile const u64 ctx_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (c *Conn) QueryContext(ctx context.Context, query string, args ...any)
SEC("uprobe/QueryContext")
int uprobe_Query_Context(struct pt_regs *ctx) {
    bpf_printk("uprobe_Query_Context !!\n");
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (c *Conn) QueryContext(ctx context.Context, query string, args ...any)
SEC("uprobe/QueryContext")
int uuprobe_Query_Context_Returns(struct pt_regs *ctx) {
    bpf_printk("uuprobe_Query_Context_Returns !!\n");
    return 0;
}