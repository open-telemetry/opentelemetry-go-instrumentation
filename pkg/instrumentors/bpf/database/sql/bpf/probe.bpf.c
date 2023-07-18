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
    u64 context_ptr_pos = 3;
    u64 query_str_ptr_pos = 4;
    u64 query_str_len_pos = 5;

    struct sql_request_t sql_request = {0};
    sql_request.start_time = bpf_ktime_get_ns();

    // Read Query string
    void *query_str_ptr = get_argument(ctx, query_str_ptr_pos);
    u64 query_str_len = (u64)get_argument(ctx, query_str_len_pos);
    u64 query_size = MAX_QUERY_SIZE < query_str_len ? MAX_QUERY_SIZE : query_str_len;
    bpf_probe_read(sql_request.query, query_size, query_str_ptr);
    // TODO : remove debug prints
    bpf_printk("query size: %d", query_size);
    bpf_printk("query_str_len: %d", query_str_len);
    bpf_printk("query string: %s", sql_request.query);

    // Get goroutine as the key fro the SQL request context
    void *goroutine = get_goroutine_address(ctx, context_ptr_pos);

    // Get parent if exists
    struct span_context *span_ctx = bpf_map_lookup_elem(&spans_in_progress, &goroutine);
    if (span_ctx != NULL) {
        // Set the parent context
        bpf_probe_read(&sql_request.psc, sizeof(sql_request.psc), span_ctx);
        copy_byte_arrays(sql_request.psc.TraceID, sql_request.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(sql_request.sc.SpanID, SPAN_ID_SIZE);
    } else {
        sql_request.sc = generate_span_context();
    }

    bpf_map_update_elem(&context_to_sql_events, &goroutine, &sql_request, 0);
    // TODO : is this realy necessery if this is a leaf
    bpf_map_update_elem(&spans_in_progress, &goroutine, &sql_request.sc, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (c *Conn) QueryContext(ctx context.Context, query string, args ...any)
SEC("uprobe/QueryContext")
int uuprobe_Query_Context_Returns(struct pt_regs *ctx) {
    bpf_printk("uuprobe_Query_Context_Returns !!\n");

    u64 context_ptr_pos = 3;
    void *goroutine = get_goroutine_address(ctx, context_ptr_pos);
    void *sqlReq_ptr = bpf_map_lookup_elem(&context_to_sql_events, &goroutine);

    // TODO : is this copy necessery (why not to cast sqlReq_ptr ?)
    struct sql_request_t sqlReq = {0};
    bpf_probe_read(&sqlReq, sizeof(sqlReq), sqlReq_ptr);
    sqlReq.end_time = bpf_ktime_get_ns();

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &sqlReq, sizeof(sqlReq));

    bpf_map_delete_elem(&context_to_sql_events, &goroutine);
    bpf_map_delete_elem(&spans_in_progress, &goroutine);
    return 0;
}