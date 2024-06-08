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

#define MAX_QUERY_SIZE 256
#define MAX_CONCURRENT 50

struct sql_request_t {
    BASE_SPAN_PROPERTIES
    char query[MAX_QUERY_SIZE];
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
    void *context_ptr_val = get_Go_context(ctx, 3, 0, true);
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
    void *key = get_consistent_key(ctx, context_ptr_val);

    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (db *DB) queryDC(ctx, txctx context.Context, dc *driverConn, releaseConn func(error), query string, args []any)
UPROBE_RETURN(queryDC, struct sql_request_t, sql_events, events, 3, 0, true, true)

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

    // Get parent if exists
    void *context_ptr_val = get_Go_context(ctx, 3, 0, true);
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
    void *key = get_consistent_key(ctx, context_ptr_val);

    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (db *DB) execDC(ctx context.Context, dc *driverConn, release func(error), query string, args []any)
UPROBE_RETURN(execDC, struct sql_request_t, sql_events, events, 3, 0, true, true)