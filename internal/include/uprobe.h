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

#ifndef _UPROBE_H_
#define _UPROBE_H_

#include "common.h"
#include "span_context.h"
#include "go_context.h"
#include "go_types.h"

#define BASE_SPAN_PROPERTIES \
    u64 start_time;          \
    u64 end_time;            \
    struct span_context sc;  \
    struct span_context psc; 

// Common flow for uprobe return:
// 1. Find consistend key for the current uprobe context
// 2. Use the key to lookup for the uprobe context in the uprobe_context_map
// 3. Update the end time of the found span
// 4. Submit the constructed event to the agent code using perf buffer events_map
// 5. Delete the span from the uprobe_context_map
// 6. Delete the span from the global active spans map
#define UPROBE_RETURN(name, event_type, uprobe_context_map, events_map, context_pos, context_offset, passed_as_arg) \
SEC("uprobe/##name##")                                                                                              \
int uprobe_##name##_Returns(struct pt_regs *ctx) {                                                                  \
    void *ctx_address = get_Go_context(ctx, context_pos, context_offset, passed_as_arg);                            \
    void *key = get_consistent_key(ctx, ctx_address);                                                               \
    void *req_ptr_map = bpf_map_lookup_elem(&uprobe_context_map, &key);                                             \
    if (req_ptr_map == NULL) {                                                                                      \
        return 0;                                                                                                   \
    }                                                                                                               \
    event_type tmpReq = {0};                                                                                        \
    bpf_probe_read(&tmpReq, sizeof(tmpReq), req_ptr_map);                                                           \
    tmpReq.end_time = bpf_ktime_get_ns();                                                                           \
    bpf_perf_event_output(ctx, &events_map, BPF_F_CURRENT_CPU, &tmpReq, sizeof(tmpReq));                            \
    bpf_map_delete_elem(&uprobe_context_map, &key);                                                                 \
    stop_tracking_span(&tmpReq.sc, &tmpReq.psc);                                                                    \
    return 0;                                                                                                       \
} 

#endif