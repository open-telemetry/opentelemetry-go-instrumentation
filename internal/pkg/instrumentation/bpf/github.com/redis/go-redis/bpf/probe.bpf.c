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
#define MAX_BATCH_SIZE 64
#define MAX_ARGS_ELE_SIZE 32

char val_str[MAX_QUERY_SIZE];

char stmt_args[MAX_ARGS_ELE_SIZE]; // single argument. Examples: `set`, `px`
char db_stmt[MAX_QUERY_SIZE]; // complete db statement, in otel it's `db.query.text`

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

// Injected in init
volatile const bool should_include_db_statement;

// func (c *baseClient) process(ctx context.Context, cmd Cmder) error
SEC("uprobe/process")
int uprobe_process(struct pt_regs *ctx) {
    struct sql_request_t sql_request = {0};

    if (should_include_db_statement) {
        // -------- GET DB STMT -------
        /*
            type StatusCmd struct {
                ctx    context.Context
                args   []interface{} <----- target field
                err    error
                keyPos int8
                _readTimeout *time.Duration
                val string
            }
        */
        // `Cmder` is iface, and the index(bytes) of iface.data is:
        // 1(*baseClient) + 2(ctx.type+ctx.data) + 2(cmd.type+cmd.data)
        void *iface_data_ptr = get_argument(ctx, 5);
        u64 args_len = 0;
        bpf_probe_read(&args_len, sizeof(args_len), (void *)(iface_data_ptr+24));
        
        u64 args_ptr = 0;
        u64 ele_ptr = 0;
        bpf_probe_read(&args_ptr, sizeof(args_ptr), (void *)(iface_data_ptr+16));
        // `MAX_BATCH_SIZE` is 64, db stmt which has more than 64 cmd segments not supported.
        // Considering the performance and limitations of eBPF, we cannot support too many segs.
        u64 db_stmt_offset = 0;
        u64 len = 0;
        #pragma unroll
        for (int i = 0; i < MAX_BATCH_SIZE; i++) {
            if (i >= args_len) {
                break;
            }
            bpf_probe_read(&ele_ptr, sizeof(ele_ptr), (void *)args_ptr);
            u64 ele_val_ptr = 0;
            bpf_probe_read(&ele_val_ptr, sizeof(ele_val_ptr), (void *)(args_ptr+8));

            __builtin_memset(stmt_args, 0, MAX_ARGS_ELE_SIZE);
            len = get_eface_true_val(stmt_args, ele_ptr, ele_val_ptr);
            args_ptr = args_ptr + 16;

            if (db_stmt_offset >= MAX_QUERY_SIZE - 1) {
                break;
            }
            if (db_stmt_offset > 0) {
                db_stmt[db_stmt_offset] = ' ';
                db_stmt_offset++;
            }
            long str_len = bpf_probe_read_str(&db_stmt[db_stmt_offset], MAX_QUERY_SIZE - db_stmt_offset, stmt_args);
            if (str_len > 0) {
                db_stmt_offset += str_len - 1;
            }
        }
        
        bpf_probe_read_kernel(sql_request.query, MAX_QUERY_SIZE, db_stmt);
    }
    sql_request.start_time = bpf_ktime_get_ns();
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

    void *key = get_consistent_key(ctx, go_context.data);
    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}

UPROBE_RETURN(process, struct sql_request_t, sql_events, events, 3, 0, true)
