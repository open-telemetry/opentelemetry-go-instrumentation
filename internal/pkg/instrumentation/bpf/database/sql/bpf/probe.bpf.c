// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_QUERY_SIZE 256
#define MAX_CONCURRENT 50

struct sql_request_t {
    BASE_SPAN_PROPERTIES
    char query[MAX_QUERY_SIZE];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct sql_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} sql_events SEC(".maps");

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

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &sql_request.psc,
        .sc = &sql_request.sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Get key
    void *key = (void *)GOROUTINE(ctx);

    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (db *DB) queryDC(ctx, txctx context.Context, dc *driverConn, releaseConn func(error), query string, args []any)
UPROBE_RETURN(queryDC, struct sql_request_t, sql_events)

// This instrumentation attaches uprobe to the following function:
// func (db *DB) execDC(ctx context.Context, dc *driverConn, release func(error), query string, args []any)
SEC("uprobe/execDC")
int uprobe_execDC(struct pt_regs *ctx) {
    // argument positions
    u64 context_ptr_pos = 3;
    u64 query_str_ptr_pos = 6;
    u64 query_str_len_pos = 7;

    struct sql_request_t sql_request = {0};
    sql_request.start_time = bpf_ktime_get_ns();

    if (should_include_db_statement) {
        // Read Query string
        void *query_str_ptr = get_argument(ctx, query_str_ptr_pos);
        u64 query_str_len = (u64)get_argument(ctx, query_str_len_pos);
        u64 query_size = MAX_QUERY_SIZE < query_str_len ? MAX_QUERY_SIZE : query_str_len;
        bpf_probe_read(sql_request.query, query_size, query_str_ptr);
    }

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &sql_request.psc,
        .sc = &sql_request.sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Get key
    void *key = (void *)GOROUTINE(ctx);

    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (db *DB) execDC(ctx context.Context, dc *driverConn, release func(error), query string, args []any)
UPROBE_RETURN(execDC, struct sql_request_t, sql_events)
