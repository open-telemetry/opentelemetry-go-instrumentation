// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
/* Copyright Authors of OpenTelemetry */

#include "common.h"
#include "bpf_helpers.h"
#include <stdbool.h>

// Injected in init
volatile const bool is_registers_abi;

void* get_argument_by_reg(struct pt_regs *ctx, int index) {
    switch (index) {
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

void* get_argument_by_stack(struct pt_regs *ctx, int index) {
    void* ptr = 0;
    bpf_probe_read(&ptr, sizeof(ptr), (void *)(ctx->rsp+(index*8)));
    return ptr;
}

void* get_argument(struct pt_regs *ctx, int index) {
    if (is_registers_abi) {
        return get_argument_by_reg(ctx, index);
    }

    return get_argument_by_stack(ctx, index);
}