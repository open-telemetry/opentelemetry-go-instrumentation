// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_CONCURRENT 50
#define MAX_SIZE 256

struct event {
	u32 size;
	char data[MAX_SIZE];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

SEC("uprobe/Span_ended")
int uprobe_Span_ended(struct pt_regs *ctx) {
	u64 len = (u64)get_argument(ctx, 3);
	if (len > MAX_SIZE) {
		bpf_printk("span data too large: %d\n", len);
		return -1;
	}
	if (len == 0) {
		bpf_printk("empty span data");
		return 0;
	}

	struct event event;
	event.size = (u32)len;

	void *data_ptr = get_argument(ctx, 2);
	if (data_ptr == NULL) {
		bpf_printk("empty span data");
		return 0;
	}

	bpf_printk("n: %d, ptr: %p, data: %d\n", event.size, data_ptr, data_ptr);

	__builtin_memset(&event.data, 0, MAX_SIZE);
	bpf_probe_read(&event.data, (u32)event.size, data_ptr);
	//bpf_probe_read_user(event.data, 95, data_ptr);

	bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));

	return 0;
}
