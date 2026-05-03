// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_OPERATION_SIZE 64
#define MAX_KEY_SIZE 64
#define MAX_ADDR_LEN 64

struct redis_request_t {
    BASE_SPAN_PROPERTIES
    char operation[MAX_OPERATION_SIZE];
    char key[MAX_KEY_SIZE];
    char address[MAX_ADDR_LEN];
    __u64 namespace;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct redis_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} redis_events SEC(".maps");

volatile const u64 args_pos;
volatile const u64 db_opts_pos;
volatile const u64 addr_opts_pos;

// This instrumentation attaches uprobe to the following function:
// func (c *baseClient) process(ctx context.Context, cmd Cmder) error
SEC("uprobe/process")
int uprobe_process(struct pt_regs *ctx) {
    // initializing the data structure to hold the extracted data
    struct redis_request_t redis_request = {};

    // retrieving the baseClient pointer and redis.Cmder instance pointer
    // arguments
    // 1 -> baseClient pointer
    // 2 -> context.Context, type pointer
    // 3 -> context.Context, data pointer
    // 4 -> redis.Cmder, type pointer
    // 5 -> redis.Cmder, data pointer
    void *client_ptr = get_argument(ctx, 1);

    void *redis_cmd_ptr = get_argument(ctx, 5);

    // reading the values from redis CMD pointer
    // step 1, ensuring that the slice has at least two members
    struct go_slice args_slice = {0};
    bpf_probe_read_user(&args_slice, sizeof(args_slice), redis_cmd_ptr+args_pos);
    if (args_slice.len < 2) {
        return 1;
    }

    // step 2, retrieving the first member of the slice, operation
    struct go_iface arg0 = {0};
    bpf_probe_read_user(&arg0, sizeof(arg0), (void *)args_slice.array);
    struct go_string arg0_string = {};
    bpf_probe_read_user(&arg0_string, sizeof(arg0_string), (void *)arg0.data);
    if (arg0_string.len <= 0 || arg0_string.len > MAX_OPERATION_SIZE) return 0;
    bpf_probe_read_user(redis_request.operation, arg0_string.len, (void *)arg0_string.str);

    // step 3, retrieving the second member of the slice, key
    struct go_iface arg1 = {0};
    bpf_probe_read_user(&arg1, sizeof(arg1), (void *)args_slice.array + sizeof(struct go_iface));
    struct go_string arg1_string = {};
    bpf_probe_read_user(&arg1_string, sizeof(arg1_string), (void *)arg1.data);
    if (arg1_string.len == 0 || arg1_string.len > MAX_OPERATION_SIZE) return 0;
    bpf_probe_read_user(redis_request.key, arg1_string.len, (void *)arg1_string.str);

    // reading values from the client pointer
    void *client_opts_ptr = NULL;
    bpf_probe_read_user(&client_opts_ptr, sizeof(client_opts_ptr), (void*)client_ptr);

    // reading the DB namespace
    if (!client_opts_ptr) return 0;
    bpf_probe_read_user(&redis_request.namespace, sizeof(redis_request.namespace), (void*)client_opts_ptr+db_opts_pos);

    // reading the address
    struct go_string address_str = {0};
    bpf_probe_read_user(&address_str, sizeof(address_str), (void*)client_opts_ptr+addr_opts_pos);
    if (address_str.len == 0 || address_str.len > MAX_ADDR_LEN) return 0;
    bpf_probe_read_user(redis_request.address, address_str.len, address_str.str);
    
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &redis_request.psc,
        .sc = &redis_request.sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Get key
    void *key = (void *)GOROUTINE(ctx);

    bpf_map_update_elem(&redis_events, &key, &redis_request, 0);

    return 0;
}

UPROBE_RETURN(process, struct redis_request_t, redis_events)
