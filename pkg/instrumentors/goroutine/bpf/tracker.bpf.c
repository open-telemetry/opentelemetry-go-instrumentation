#include "arguments.h"
#include "goroutines.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Injected in init
volatile const u64 goid_pos;

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus(struct pt_regs *ctx) {
    s32 newval = 0;
    bpf_probe_read(&newval, sizeof(newval), (void*)(ctx->rsp+20));
    if (newval != 2) {
        return 0;
    }

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
    s32 newval = (s32)(ctx->rcx);
    if (newval != 2) {
        return 0;
    }

    void* g_ptr = (void *)(ctx->rax);
    s64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), g_ptr+goid_pos);
    u64 current_thread = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&goroutines_map, &current_thread, &goid, 0);

    return 0;
}