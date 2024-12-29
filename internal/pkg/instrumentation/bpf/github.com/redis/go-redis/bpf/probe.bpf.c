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
#define MAX_RESP_BUF_SIZE 256
#define MAX_SUBCMD_CNT 10

struct sql_request_t {
    BASE_SPAN_PROPERTIES
    u8  resp_msg[MAX_QUERY_SIZE];
    int segs; // segs only be set in redis pipeline mode
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct sql_request_t);
	__uint(max_entries, MAX_CONCURRENT);
} sql_events SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, void*);
    __type(value, void*);
    __uint(max_entries, MAX_CONCURRENT);
} writer_conn SEC(".maps");

// Storage the segments of db stmt. Example: `set name alice px 2`, the segments is 5.
struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u64));
    __uint(max_entries, 1);
} stmt_segments SEC(".maps");

// Injected in init
volatile const bool should_include_db_statement;

// func (c *baseClient) generalProcessPipeline(ctx context.Context, cmds []Cmder, p pipelineProcessor) error
SEC("uprobe/processPipeline")
int uprobe_processPipeline(struct pt_regs *ctx) {
    if (!should_include_db_statement) {
        return 0;
    }
    u64 cmds_len_pos = 5;
    u64 cmds_len = (u64)get_argument(ctx, cmds_len_pos);
    
    u64 cmds_ptr_pos = 4;
    u64 cmds_ptr = (u64)get_argument(ctx, cmds_ptr_pos);

    u64 ele_ptr = 0;
    u64 segs = 0;
    for (u64 i = 0; i < MAX_SUBCMD_CNT; i++) {
        if (i >= cmds_len) {
            break;
        }
        // 8 = iface.tab
        bpf_probe_read(&ele_ptr, sizeof(ele_ptr), (void *)(cmds_ptr+8));
        u64 subcmd_segs = 0;
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
        // 24 = 16(StatusCmd.ctx) + 8(StatusCmd.args.array)
        bpf_probe_read(&subcmd_segs, sizeof(subcmd_segs), (void *)(ele_ptr+24));
        cmds_ptr += 16;
        segs += subcmd_segs;
    }
    u32 map_id = 0;
    bpf_map_update_elem(&stmt_segments, &map_id, &segs, BPF_ANY);
    return 0;
}

// func (cn *Conn) WithWriter(ctx context.Context, timeout time.Duration, fn func(wr *proto.Writer) error)
SEC("uprobe/WithWriter")
int uprobe_WithWriter(struct pt_regs *ctx) {
    void *conn_ptr = get_argument(ctx, 1);
    
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    
    struct sql_request_t sql_request = {0};
    sql_request.start_time = bpf_ktime_get_ns();
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
    bpf_map_update_elem(&writer_conn, &key, &conn_ptr, 0);
    bpf_map_update_elem(&sql_events, &key, &sql_request, 0);
    return 0;
}


// func (cn *Conn) WithWriter(ctx context.Context, timeout time.Duration, fn func(wr *proto.Writer) error)
SEC("uprobe/WithWriter")
int uprobe_WithWriter_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();

    int segs = 0;
    u32 map_id = 0;
    u64 *value;
    value = bpf_map_lookup_elem(&stmt_segments, &map_id);
    if (value == NULL) {
        bpf_printk("map stmt_segments lookup failed");
    } else {
        segs = (int)*value;
    }

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    void *key = get_consistent_key(ctx, go_context.data);
    void **conn_ptr = bpf_map_lookup_elem(&writer_conn, &key);
    if (!conn_ptr) {
        bpf_printk("map writer_conn lookup failed");
        return 0;
    }

    u64 bw_offset = 32;
    u64 bw_ptr;
    bpf_probe_read(&bw_ptr, sizeof(bw_ptr), (void *)(*conn_ptr+bw_offset));

    u64 buf_ele_ptr;
    bpf_probe_read(&buf_ele_ptr, sizeof(buf_ele_ptr), (void *)(bw_ptr + 16));

    struct sql_request_t *sql_request = bpf_map_lookup_elem(&sql_events, &key);
    if (!sql_request) {
        bpf_printk("[uprobe_WithWriter_Returns] map sql_request_t looup failed");
        return 0;
    }

    // Only obtain resp buf when necessary
    if (should_include_db_statement) {
        u8 buf_ele;
        for (u64 i = 0; i < MAX_RESP_BUF_SIZE; i++) {
            bpf_probe_read(&buf_ele, sizeof(buf_ele), (void *)buf_ele_ptr);
            sql_request->resp_msg[i] = buf_ele;
            buf_ele_ptr += 1;
        }
    }
    sql_request->segs = segs;
    sql_request->end_time = end_time;
    output_span_event(ctx, sql_request, sizeof(*sql_request), &sql_request->sc);
    stop_tracking_span(&sql_request->sc, &sql_request->psc);
    bpf_map_delete_elem(&writer_conn, &key);

    u64 zero = 0;
    bpf_map_update_elem(&stmt_segments, &map_id, &zero, BPF_ANY);
    return 0;
}
