// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef _ALLOC_H_
#define _ALLOC_H_

#include "bpf_helpers.h"

#define MAX_ENTRIES 50
#define MAX_BUFFER_SIZE 1024
#define MIN_BUFFER_SIZE 8

// Injected in init
volatile const u32 total_cpus;
volatile const u64 start_addr;
volatile const u64 end_addr;

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_HASH);
    __type(key, s32);
    __type(value, u64);
    __uint(max_entries, MAX_ENTRIES);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} alloc_map SEC(".maps");

// Buffer for aligned data
struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, MAX_BUFFER_SIZE);
    __uint(max_entries, 1);
} alignment_buffer SEC(".maps");

static __always_inline u64 get_area_start()
{
    s64 partition_size = (end_addr - start_addr) / total_cpus;
    u32 current_cpu = bpf_get_smp_processor_id();
    s32 start_index = 0;
    u64 *start = (u64 *)bpf_map_lookup_elem(&alloc_map, &start_index);
    if (start == NULL || *start == 0)
    {
        u64 current_start_addr = start_addr + (partition_size * current_cpu);
        bpf_map_update_elem(&alloc_map, &start_index, &current_start_addr, BPF_ANY);
        return current_start_addr;
    }
    else
    {
        return *start;
    }
}

static __always_inline u64 get_area_end(u64 start)
{
    s64 partition_size = (end_addr - start_addr) / total_cpus;
    s32 end_index = 1;
    u64 *end = (u64 *)bpf_map_lookup_elem(&alloc_map, &end_index);
    if (end == NULL || *end == 0)
    {
        u64 current_end_addr = start + partition_size;
        bpf_map_update_elem(&alloc_map, &end_index, &current_end_addr, BPF_ANY);
        return current_end_addr;
    }
    else
    {
        return *end;
    }
}

static __always_inline s32 bound_number(s32 num, s32 min, s32 max)
{
    if (num < min)
    {
        return min;
    }
    else if (num > max)
    {
        return max;
    }
    return num;
}

static __always_inline void *write_target_data(void *data, s32 size)
{
    if (!data || data == NULL)
    {
        return NULL;
    }

     // Add padding to align to 8 bytes
    if (size % 8 != 0) {
        size += 8 - (size % 8);

        // Write to the buffer
        u32 key = 0;
        void *buffer = bpf_map_lookup_elem(&alignment_buffer, &key);
        if (buffer == NULL) {
            bpf_printk("failed to get alignment buffer");
            return NULL;
        }

        // Copy size bytes from data to buffer
        size = bound_number(size, MIN_BUFFER_SIZE, MAX_BUFFER_SIZE);
        long success = bpf_probe_read(buffer, size, data);
        if (success != 0) {
            bpf_printk("failed to copy data to alignment buffer");
            return NULL;
        }

        data = buffer;
    }

    u64 start = get_area_start();
    u64 end = get_area_end(start);
    if (end - start < size)
    {
        bpf_printk("reached end of CPU memory block, going to the start again");
        s32 start_index = 0;
        bpf_map_delete_elem(&alloc_map, &start_index);
        start = get_area_start();
    }

    void *target = (void *)start;
    size = bound_number(size, MIN_BUFFER_SIZE, MAX_BUFFER_SIZE);

    u64 distance_from_start_addr = (u64)target - start_addr;
    u64 distance_from_next_page = 4096 - (distance_from_start_addr % 4096);
    if (distance_from_next_page < size)
    {
        target += distance_from_next_page + 1;
    } else if (distance_from_next_page == 4096) {
        target += 1;
    }

    long success = bpf_probe_write_user(target, data, size);
    if (success == 0)
    {
        s32 start_index = 0;
        u64 updated_start = start + size;

        bpf_map_update_elem(&alloc_map, &start_index, &updated_start, BPF_ANY);
        return target;
    }
    else
    {
        bpf_printk("failed to write to userspace, error code: %d, addr: %lx, next_page_distance: %d", success, target, distance_from_next_page);
    }
    return NULL;
}

#endif
