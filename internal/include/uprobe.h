// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _UPROBE_H_
#define _UPROBE_H_

#include "common.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "trace/span_output.h"

#define BASE_SPAN_PROPERTIES                                                                       \
    u64 start_time;                                                                                \
    u64 end_time;                                                                                  \
    struct span_context sc;                                                                        \
    struct span_context psc;

// Common flow for uprobe return:
// 1. Find consistent key for the current uprobe context
// 2. Use the key to lookup for the uprobe context in the uprobe_context_map
// 3. Update the end time of the found span
// 4. Submit the constructed event to the agent code using perf buffer events_map
// 5. Delete the span from the global active spans map (in case the span is not tracked in the active spans map, this will be a no-op)
// 6. Delete the span from the uprobe_context_map
#define UPROBE_RETURN(name, event_type, uprobe_context_map)                                        \
    SEC("uprobe/##name##")                                                                         \
    int uprobe_##name##_Returns(struct pt_regs *ctx) {                                             \
        void *key = (void *)GOROUTINE(ctx);                                                        \
        event_type *event = bpf_map_lookup_elem(&uprobe_context_map, &key);                        \
        if (event == NULL) {                                                                       \
            bpf_printk("event is NULL in ret probe");                                              \
            return 0;                                                                              \
        }                                                                                          \
        event->end_time = bpf_ktime_get_ns();                                                      \
        output_span_event(ctx, event, sizeof(event_type), &event->sc);                             \
        stop_tracking_span(&event->sc, &event->psc);                                               \
        bpf_map_delete_elem(&uprobe_context_map, &key);                                            \
        return 0;                                                                                  \
    }

#endif
