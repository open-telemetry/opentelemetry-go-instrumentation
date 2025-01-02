// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "trace/start_span.h"
#include "go_net.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_CONCURRENT 56 // todo: tune
#define MAX_OPERATION_NAME_SIZE 20

typedef struct tcp_addr {
    u8 ip[16];
    u64 port;
} tcp_addr_t;

struct rueidis_completed_command_t {
    BASE_SPAN_PROPERTIES
    char operation_name[MAX_OPERATION_NAME_SIZE];
    net_addr_t local_addr;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void*);
    __type(value, struct rueidis_completed_command_t);
    __uint(max_entries, MAX_CONCURRENT);
} redis_completed_events SEC(".maps");

volatile const u64 pipe_conn_pos;
volatile const u64 tcp_conn_conn_pos;
volatile const u64 conn_fd_pos;
volatile const u64 fd_raddr_pos;
volatile const u64 tcp_addr_ip_pos;
volatile const u64 tcp_addr_port_pos;
volatile const u64 completed_cs_pos;
volatile const u64 cs_s_pos;
volatile const u64 result_error_pos;

const u64 max_opration_length = MAX_OPERATION_NAME_SIZE;

// This instrumentation attaches uprobe to the following function:
// func (m *pipe) Do(ctx context.Context, cmd Completed) (resp RedisResult)
SEC("uprobe/pipe_Do")
int uprobe_pipe_Do(struct pt_regs *ctx) {
  	int cmd_cs_ptr_pos = 4; // passed in $rdi
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);
    void *redisReq_ptr = bpf_map_lookup_elem(&redis_completed_events, &key);
    if (redisReq_ptr != NULL) {
        bpf_printk("uprobe/pipe_Do already tracked with the current context");
        return 0;
    }

    struct rueidis_completed_command_t redisReq = {};
    redisReq.start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
            .ctx = ctx,
            .go_context = &go_context,
            .psc = &redisReq.psc,
            .sc = &redisReq.sc,
            .get_parent_span_context_fn = NULL,
            .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);


    // read network peer data. located at pipe.conn.conn.fd.raddr
    void *pipe_ptr = get_argument(ctx, 1);
    // todo: check pipe not having conn scenario for older version

    void *tcp_conn_ptr = 0;
    bpf_probe_read_user(&tcp_conn_ptr, sizeof(tcp_conn_ptr), get_go_interface_instance(pipe_ptr + pipe_conn_pos));


    void *fd_ptr = 0;
    bpf_probe_read_user(&fd_ptr, sizeof(fd_ptr), (void *)(tcp_conn_ptr + tcp_conn_conn_pos + conn_fd_pos));

    void *tcp_addr_ptr = 0;
    bpf_probe_read_user(&tcp_addr_ptr, sizeof(tcp_addr_ptr),  get_go_interface_instance(fd_ptr + fd_raddr_pos));

    get_tcp_net_addr_from_tcp_addr(ctx, &redisReq.local_addr, (void *)(tcp_addr_ptr));
    // port still has issues. its type conversion with golang controller though.

	// read redis command's operation from cmd.cs[0]
    void *cs_ptr = get_argument(ctx, cmd_cs_ptr_pos);
    if (cs_ptr == 0) {
        goto done;
    }

    struct go_slice cs = {0};
    bpf_probe_read(&cs, sizeof(cs), cs_ptr);

	if (!get_go_string_from_user_ptr(cs.array, &redisReq.operation_name, max_opration_length)) {
        bpf_printk("uprobe/pipe_Do command from Completed.cs.s");
    }

    // todo: get full query text

    done:

    bpf_map_update_elem(&redis_completed_events, &key, &redisReq, 0);
    start_tracking_span(go_context.data, &redisReq.sc);

    return 0;
}


// This instrumentation attaches uretprobe to the following function:
// func (m *pipe) Do(ctx context.Context, cmd Completed) (resp RedisResult)
SEC("uprobe/pipe_Do")
int uprobe_pipe_Do_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();

    struct go_iface go_context = {0};
    get_Go_context(ctx, 3, 0, true, &go_context);

    void *key = get_consistent_key(ctx, go_context.data);
    struct rueidis_completed_command_t *redisReq = bpf_map_lookup_elem(&redis_completed_events, &key);

    if (redisReq == NULL) {
        bpf_printk("event is NULL in ret probe");
        return 0;
    }

    // todo: check if error happened

    done:
    redisReq->end_time = bpf_ktime_get_ns();
    output_span_event(ctx, redisReq, sizeof(*redisReq), &redisReq->sc);
    stop_tracking_span(&redisReq->sc, &redisReq->psc);

    bpf_map_delete_elem(&redis_completed_events, &key);
    return 0;
}
