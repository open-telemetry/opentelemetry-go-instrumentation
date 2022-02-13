#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define runningState 2

struct goroutine_event {
    s64 goroutine_id;
    u32 thread_id;
    u64 timestamp;
};


struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, s64);
	__uint(max_entries, MAX_OS_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, 1);
} offset_map SEC(".maps");

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus(struct pt_regs *ctx) {
    u32 newval = 0;
    bpf_probe_read(&newval, sizeof(newval), (void*)(ctx->rsp+20));
    if (newval != runningState) {
        return 0;
    }

    u32 offset_key = 0;
    u64* offset_ptr = bpf_map_lookup_elem(&offset_map, &offset_key);
    if (!offset_ptr) {
        return 0;
    }

    u64 offset = 0;
    bpf_probe_read(&offset, sizeof(offset), offset_ptr);
    void* g_ptr_ptr = (void *)(ctx->rsp+8);
    void* g_ptr;
    bpf_probe_read(&g_ptr, sizeof(g_ptr), g_ptr_ptr);
    s64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), g_ptr+offset);
    u32 id = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&goroutines_map, &id, &goid, 0);
    return 0;
}