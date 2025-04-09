// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _GO_TYPES_H
#define _GO_TYPES_H

#include "utils.h"
#include "alloc.h"
#include "bpf_helpers.h"

/* Max size of slice array in bytes 
 Keep a power of 2 to help with masks */
#define MAX_SLICE_ARRAY_SIZE 1024

typedef struct go_string
{
    char *str;
    s64 len;
} go_string_t;

typedef struct go_slice
{
    void *array;
    s64 len;
    s64 cap;
} go_slice_t;

typedef struct go_iface
{
    void *type;
    void *data;
} go_iface_t;

// a map bucket type with the given key and value types
#define MAP_BUCKET_TYPE(key_type, value_type) struct map_bucket_##key_type##_##value_type##_t
// a map bucket struct definition with the given key and value types
// for more details about the structure of a map bucket see:
// https://github.com/golang/go/blob/639cc0dcc0948dd02c9d5fc12fbed730a21ebebc/src/runtime/map.go#L143
#define MAP_BUCKET_DEFINITION(key_type, value_type) \
MAP_BUCKET_TYPE(key_type, value_type) { \
    char tophash[8]; \
    key_type keys[8]; \
    value_type values[8]; \
    void *overflow; \
};

struct slice_array_buff
{
    unsigned char buff[MAX_SLICE_ARRAY_SIZE];
};

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, u32);
    __type(value, struct slice_array_buff);
    __uint(max_entries, 1);
} slice_array_buff_map SEC(".maps");

// In Go, interfaces are represented as a pair of pointers: a pointer to the
// interface data, and a pointer to the interface table.
// See: runtime.iface in https://golang.org/src/runtime/runtime2.go
static __always_inline void* get_go_interface_instance(void *iface)
{
    return (void*)(iface + 8);
}

static __always_inline struct go_string write_user_go_string(char *str, u32 len)
{
    // Copy chars to userspace
    struct go_string new_string = {.str = NULL, .len = 0};
    char *addr = write_target_data((void *)str, len);
    if (addr == NULL) {
        bpf_printk("write_user_go_string: failed to copy string to userspace");
        return new_string;
    }

    // Build string struct in kernel space
    new_string.str = addr;
    new_string.len = len;

    // Copy new string struct to userspace
    void *res = write_target_data((void *)&new_string, sizeof(new_string));
    if (res == NULL) {
        new_string.len = 0;
    }

    return new_string;
}

static __always_inline void append_item_to_slice(void *new_item, u32 item_size, void *slice_user_ptr)
{
    // read the slice descriptor
    struct go_slice slice = {0};
    bpf_probe_read(&slice, sizeof(slice), slice_user_ptr);
    long res = 0;

    u64 slice_len = slice.len;
    u64 slice_cap = slice.cap;
    if (slice_len < slice_cap && slice.array != NULL)
    {
        // Room available on current array, append to the underlying array
        res = bpf_probe_write_user(slice.array + (item_size * slice_len), new_item, item_size);
    }
    else
    { 
        // No room on current array - try to copy new one of size item_size * (len + 1)
        u32 alloc_size = item_size * slice_len;
        if (alloc_size >= MAX_SLICE_ARRAY_SIZE)
        {
            return;
        }
    
        // Get temporary buffer
        u32 index = 0;
        struct slice_array_buff *map_buff = bpf_map_lookup_elem(&slice_array_buff_map, &index);
        if (!map_buff)
        {
            return;
        }
    
        unsigned char *new_slice_array = map_buff->buff;
        // help the verifier
        alloc_size &= (MAX_SLICE_ARRAY_SIZE - 1);
        if (alloc_size + item_size > MAX_SLICE_ARRAY_SIZE)
        {
            // No room for new item
            return;
        }
        // Append to buffer
        if (slice.array != NULL) {
            bpf_probe_read_user(new_slice_array, alloc_size, slice.array);
            bpf_printk("append_item_to_slice: copying %d bytes to new array from address 0x%llx", alloc_size, slice.array);
        }
        copy_byte_arrays(new_item, new_slice_array + alloc_size, item_size);

        // Copy buffer to userspace
        u32 new_array_size = alloc_size + item_size;

        void *new_array = write_target_data(new_slice_array, new_array_size);
        if (new_array == NULL)
        {
            bpf_printk("append_item_to_slice: failed to copy new array to userspace");
            return;
        }

        // Update array pointer of slice
        slice.array = new_array;
        slice.cap++;
    }

    // Update len
    slice.len++;
    long success = bpf_probe_write_user(slice_user_ptr, &slice, sizeof(slice));
    if (success != 0)
    {
        bpf_printk("append_item_to_slice: failed to update slice in userspace");
        return;
    }
}

static __always_inline bool get_go_string_from_user_ptr(void *user_str_ptr, char *dst, u64 max_len)
{
    if (user_str_ptr == NULL || max_len == 0) {
        return false;
    }

    struct go_string user_str = {0};
    long success = 0;
    success = bpf_probe_read_user(&user_str, sizeof(struct go_string), user_str_ptr);
    if (success != 0 || user_str.len < 1) {
        return false;
    }

    u64 size_to_read = user_str.len > max_len ? max_len : user_str.len;
    __builtin_memset(dst, 0, max_len);
    
    success = bpf_probe_read_user(dst, size_to_read, user_str.str);
    if (success != 0) {
        return false;
    }
    return true;
}
#endif
