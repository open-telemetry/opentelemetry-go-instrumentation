#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define runningState 2

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, s64);
	__uint(max_entries, MAX_OS_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

// Injected in init
volatile const u64 goid_pos;

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus(struct pt_regs *ctx) {
    void* g_ptr;
    bpf_probe_read(&g_ptr, sizeof(g_ptr), (void *)(ctx->rsp+8));
    s64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), g_ptr+goid_pos);
    u64 current_thread = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&goroutines_map, &current_thread, &goid, 0);
    return 0;
}

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus_ByRegisters(struct pt_regs *ctx) {
    void* g_ptr = (void *)(ctx->rax);
    s64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), g_ptr+goid_pos);
    u64 current_thread = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&goroutines_map, &current_thread, &goid, 0);
    return 0;
}