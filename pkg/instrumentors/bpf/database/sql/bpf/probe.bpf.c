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
} sql_events SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const bool should_include_db_statement;

// This instrumentation attaches uprobe to the following function:
// func (db *DB) queryDC(ctx, txctx context.Context, dc *driverConn, releaseConn func(error), query string, args []any)
SEC("uprobe/queryDC")
int uprobe_queryDC(struct pt_regs *ctx) {
    // argument positions
    u64 context_ptr_pos = 3;
    u64 query_str_ptr_pos = 8;
    u64 query_str_len_pos = 9;

    struct sql_request_t sql_request = {0};
    sql_request.start_time = bpf_ktime_get_ns();

    if (should_include_db_statement) {
        // Read Query string
        void *query_str_ptr = get_argument(ctx, query_str_ptr_pos);
        u64 query_str_len = (u64)get_argument(ctx, query_str_len_pos);
        u64 query_size = MAX_QUERY_SIZE < query_str_len ? MAX_QUERY_SIZE : query_str_len;
        bpf_probe_read(sql_request.query, query_size, query_str_ptr);
    }

    // Get parent if exists
    void *context_ptr = get_argument(ctx, context_ptr_pos);
    void *context_ptr_val = 0;
    bpf_probe_read(&context_ptr_val, sizeof(context_ptr_val), context_ptr);
    struct span_context *span_ctx = get_parent_span_context(context_ptr_val);
    if (span_ctx != NULL) {
        // Set the parent context
        bpf_probe_read(&sql_request.psc, sizeof(sql_request.psc), span_ctx);
        copy_byte_arrays(sql_request.psc.TraceID, sql_request.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(sql_request.sc.SpanID, SPAN_ID_SIZE);
    } else {
        sql_request.sc = generate_span_context();
    }

    // Get key
    void *key = get_consistent_key(ctx, context_ptr);

    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    start_tracking_span(context_ptr_val, &sql_request.sc);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (db *DB) queryDC(ctx, txctx context.Context, dc *driverConn, releaseConn func(error), query string, args []any)
SEC("uprobe/queryDC")
int uprobe_queryDC_Returns(struct pt_regs *ctx) {
    u64 context_ptr_pos = 3;
    // Find the corresponding sql event we return from
    void *context_ptr = get_argument(ctx, context_ptr_pos);
    void *key = get_consistent_key(ctx, context_ptr);
    void *sqlReq_ptr = bpf_map_lookup_elem(&sql_events, &key);

    struct sql_request_t sqlReq = {0};
    bpf_probe_read(&sqlReq, sizeof(sqlReq), sqlReq_ptr);
    sqlReq.end_time = bpf_ktime_get_ns();

    // Send event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &sqlReq, sizeof(sqlReq));
    // Clean the sql event
    bpf_map_delete_elem(&sql_events, &key);
    stop_tracking_span(&sqlReq.sc);
    return 0;
}