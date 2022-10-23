#include "bpf_helpers.h"

#define MAX_DISTANCE 10

static __always_inline void* find_context_in_map(void *ctx, void *context_map) {
    void *data = ctx;
    for (int i = 0; i < MAX_DISTANCE; i++) {
        void* found_in_map = bpf_map_lookup_elem(context_map, &data);
        if (found_in_map != NULL) {
            return data;
        }

        // We assume context.Context implementation containens Parent context.Context member
        // Since the parent is also an interface, we need to read the data part of it
        bpf_probe_read(&data, sizeof(data), data+8);
    }

    bpf_printk("context %lx not found in context map", ctx);
    return NULL;
}