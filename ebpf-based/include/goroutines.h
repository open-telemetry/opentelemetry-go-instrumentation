#include "bpf_helpers.h"

#define MAX_SYSTEM_THREADS 20

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, s64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

s64 get_current_goroutine() {
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    s64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    return goid;
}