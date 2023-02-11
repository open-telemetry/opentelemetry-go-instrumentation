#include "bpf_helpers.h"

#define MAX_ENTRIES 50

// Injected in init
volatile const u32 total_cpus;
volatile const u64 start_addr;
volatile const u64 end_addr;

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, s32);
	__type(value, u64);
	__uint(max_entries, MAX_ENTRIES);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} alloc_map SEC(".maps");

static __always_inline u64 get_area_start() {
    s64 partition_size = (end_addr - start_addr) / total_cpus;
    u32 current_cpu = bpf_get_smp_processor_id();
    s32 start_index = 0;
    u64* start = (u64*) bpf_map_lookup_elem(&alloc_map, &start_index);
    if (start == NULL || *start == 0) {
        u64 current_start_addr = start_addr + (partition_size * current_cpu);
        bpf_map_update_elem(&alloc_map, &start_index, &current_start_addr, BPF_ANY);
        return current_start_addr;
    } else {
        return *start;
    }
}

static __always_inline u64 get_area_end(u64 start) {
    s64 partition_size = (end_addr - start_addr) / total_cpus;
    s32 end_index = 1;
    bpf_printk("total size: %d, partition size: %d, modulo: %d", end_addr - start_addr, partition_size, (end_addr - start_addr) % partition_size);
    u64* end = (u64*)bpf_map_lookup_elem(&alloc_map, &end_index);
    if (end == NULL || *end == 0) {
        u64 current_end_addr = start + partition_size;
        bpf_map_update_elem(&alloc_map, &end_index, &current_end_addr, BPF_ANY);
        return current_end_addr;
    } else {
        return *end;
    }
}

static __always_inline void* write_target_data(void* data, s32 size) {
    if (!data || data == NULL) {
        return NULL;
    }

    u64 start = get_area_start();
    u64 end = get_area_end(start);
    s32 current_cpu = bpf_get_smp_processor_id();
    if (end - start < size) {
        bpf_printk("reached end of CPU memory block, going to the start again");
        s32 start_index = 0;
        bpf_map_delete_elem(&alloc_map, &start_index);
        start = get_area_start();
    }

    void* target = (void*)start;
    long success = bpf_probe_write_user(target, data, size);
    if (success == 0) {
        s32 start_index = 0;
        u64 updated_start = start + size;

        // align updated_start to 8 bytes
        if (updated_start % 8 != 0) {
            updated_start += 8 - (updated_start % 8);
        }

        bpf_map_update_elem(&alloc_map, &start_index, &updated_start, BPF_ANY);
        return target;
    }

    bpf_printk("failed to write to userspace, error code: %d, addr: %lx", success, target);
    return NULL;
}