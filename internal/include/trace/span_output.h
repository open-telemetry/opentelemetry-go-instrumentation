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

#include "trace/span_context.h"
#include "common.h"
#include "trace/span_context.h"

#ifndef _SPAN_OUTPUT_H_
#define _SPAN_OUTPUT_H_

struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Output a record to the perf buffer. If the span context is sampled, the record is outputted.
// Returns 0 on success, negative error code on failure.
static __always_inline long output_span_event(void *ctx, void *data, u64 size, struct span_context *sc) {
    bool sampled = (sc != NULL && is_sampled(sc));
    if (sampled) {
        return bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, data, size);
    }
    return 0;
}

#endif