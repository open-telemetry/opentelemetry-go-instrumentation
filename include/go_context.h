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

#include "bpf_helpers.h"

#define MAX_DISTANCE 10

static __always_inline void *find_context_in_map(void *ctx, void *context_map)
{
    void *data = ctx;
    for (int i = 0; i < MAX_DISTANCE; i++)
    {
        void *found_in_map = bpf_map_lookup_elem(context_map, &data);
        if (found_in_map != NULL)
        {
            return data;
        }

        // We assume context.Context implementation containens Parent context.Context member
        // Since the parent is also an interface, we need to read the data part of it
        bpf_probe_read(&data, sizeof(data), data + 8);
    }

    bpf_printk("context %lx not found in context map", ctx);
    return NULL;
}
