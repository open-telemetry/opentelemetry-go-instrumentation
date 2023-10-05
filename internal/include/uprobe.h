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
#define UPROBE_RETURN(name, event_type, ctx_struct_pos, ctx_struct_offset, uprobe_context_map, events_map, is_root) \
SEC("uprobe/##name##")                                                                                              \
int uprobe_##name##_Returns(struct pt_regs *ctx) {                                                                  \
    void *req_ptr = get_argument(ctx, ctx_struct_pos);                                                              \
    void *key = get_consistent_key(ctx, (void *)(req_ptr + ctx_struct_offset));                                     \
    void *req_ptr_map = bpf_map_lookup_elem(&uprobe_context_map, &key);                                             \
    event_type tmpReq = {};                                                                                         \
    bpf_probe_read(&tmpReq, sizeof(tmpReq), req_ptr_map);                                                           \
    tmpReq.end_time = bpf_ktime_get_ns();                                                                           \
    bpf_perf_event_output(ctx, &events_map, BPF_F_CURRENT_CPU, &tmpReq, sizeof(tmpReq));                            \
    bpf_map_delete_elem(&uprobe_context_map, &key);                                                                 \
    bool is_local_root = (is_root || bpf_is_zero(&tmpReq.psc, sizeof(tmpReq.psc)));                                 \
    stop_tracking_span(&tmpReq.sc, is_local_root);                                                                  \
    return 0;                                                                                                       \
} 

#endif