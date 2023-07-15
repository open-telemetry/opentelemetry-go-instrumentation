#include "arguments.h"
#include "span_context.h"
#include "go_context.h"
#include "go_types.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_QUERY_SIZE 100
#define MAX_CONCURRENT 50

struct sql_request_t {
    u64 start_time;
    u64 end_time;
    char query[MAX_QUERY_SIZE];
    struct span_context sc;
    struct span_context psc;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct sql_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} context_to_sql_events SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");


// This instrumentation attaches uprobe to the following function:
// func (c *Conn) QueryContext(ctx context.Context, query string, args ...any)
SEC("uprobe/QueryContext")
int uprobe_Query_Context(struct pt_regs *ctx) {
    bpf_printk("uprobe_Query_Context !!\n");
    // argument positions
    u64 query_str_ptr_pos = 4;
    u64 query_str_len_pos = 5;

    struct sql_request_t sql_request = {0};
    sql_request.start_time = bpf_ktime_get_ns();

    bpf_printk("arg 1: 0x%lx", get_argument(ctx, 1));
    bpf_printk("arg 2: 0x%lx", get_argument(ctx, 2));
    bpf_printk("arg 3: 0x%lx", get_argument(ctx, 3));
    bpf_printk("arg 4: 0x%lx", get_argument(ctx, 4));
    bpf_printk("arg 5: 0x%lx", get_argument(ctx, 5));

    // Read Query string
    void *query_str_ptr = get_argument(ctx, query_str_ptr_pos);
    u64 query_str_len = (u64)get_argument(ctx, query_str_len_pos);
    u64 query_size = MAX_QUERY_SIZE < query_str_len ? MAX_QUERY_SIZE : query_str_len;
    bpf_probe_read(sql_request.query, query_size, query_str_ptr);
    bpf_printk("query size: %d", query_size);
    bpf_printk("query_str_len: %d", query_str_len);
    bpf_printk("query string: %s", sql_request.query);

    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (c *Conn) QueryContext(ctx context.Context, query string, args ...any)
SEC("uprobe/QueryContext")
int uuprobe_Query_Context_Returns(struct pt_regs *ctx) {
    bpf_printk("uuprobe_Query_Context_Returns !!\n");
    return 0;
}