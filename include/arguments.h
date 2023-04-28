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

#include "common.h"
#include "bpf_helpers.h"
#include <stdbool.h>

// Injected in init
volatile const bool is_registers_abi;

void *get_argument_by_reg(struct pt_regs *ctx, int index)
{
    switch (index)
    {
    case 1:
        return (void *)(ctx->rax);
    case 2:
        return (void *)(ctx->rbx);
    case 3:
        return (void *)(ctx->rcx);
    case 4:
        return (void *)(ctx->rdi);
    case 5:
        return (void *)(ctx->rsi);
    case 6:
        return (void *)(ctx->r8);
    case 7:
        return (void *)(ctx->r9);
    case 8:
        return (void *)(ctx->r10);
    case 9:
        return (void *)(ctx->r11);
    default:
        return NULL;
    }
}

void *get_argument_by_stack(struct pt_regs *ctx, int index)
{
    void *ptr = 0;
    bpf_probe_read(&ptr, sizeof(ptr), (void *)(ctx->rsp + (index * 8)));
    return ptr;
}

void *get_argument(struct pt_regs *ctx, int index)
{
    if (is_registers_abi)
    {
        return get_argument_by_reg(ctx, index);
    }

    return get_argument_by_stack(ctx, index);
}

// Every span created by the auto instrumentation should contain end timestamp.
// This end timestamp is recorded at the end of probed function by editing the struct that was created at the beginning.
// Usually instrumentors create an eBPF map to store the span struct and retrieve it at the end of the function.
// Consistent key is used as a key to the map.
// For Go < 1.17: consistent key is the address of context.Context.
// For Go >= 1.17: consistent key is the goroutine address.
static __always_inline void *get_consistent_key(struct pt_regs *ctx, void *contextContext)
{
    if (is_registers_abi)
    {
        // TODO: replace with GOROUTINE macro once ARM PR merged
        return (void *)(ctx->r14);
    }

    return contextContext;
}