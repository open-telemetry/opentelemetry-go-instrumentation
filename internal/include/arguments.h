// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _ARGUMENTS_H_
#define _ARGUMENTS_H_

#include "common.h"
#include "bpf_tracing.h"
#include "bpf_helpers.h"
#include <stdbool.h>

#if defined(bpf_target_x86)

#if defined(__KERNEL__) || defined(__VMLINUX_H__)

// https://go.googlesource.com/go/+/refs/heads/dev.regabi/src/cmd/compile/internal-abi.md#amd64-architecture

#define GO_PARAM1(x) ((x)->ax)
#define GO_PARAM2(x) ((x)->bx)
#define GO_PARAM3(x) ((x)->cx)
#define GO_PARAM4(x) ((x)->di)
#define GO_PARAM5(x) ((x)->si)
#define GO_PARAM6(x) ((x)->r8)
#define GO_PARAM7(x) ((x)->r9)
#define GO_PARAM8(x) ((x)->r10)
#define GO_PARAM9(x) ((x)->r11)
#define GOROUTINE(x) ((x)->r14)

#endif

#elif defined(bpf_target_arm64)

// https://github.com/golang/go/blob/45447b4bfff4227a8945951dd7d37f2873992e1b/src/cmd/compile/abi-internal.md#arm64-architecture

#define GO_PARAM1(x) (__PT_REGS_CAST(x)->__PT_PARM1_REG)
#define GO_PARAM2(x) (__PT_REGS_CAST(x)->__PT_PARM2_REG)
#define GO_PARAM3(x) (__PT_REGS_CAST(x)->__PT_PARM3_REG)
#define GO_PARAM4(x) (__PT_REGS_CAST(x)->__PT_PARM4_REG)
#define GO_PARAM5(x) (__PT_REGS_CAST(x)->__PT_PARM5_REG)
#define GO_PARAM6(x) (__PT_REGS_CAST(x)->__PT_PARM6_REG)
#define GO_PARAM7(x) (__PT_REGS_CAST(x)->__PT_PARM7_REG)
#define GO_PARAM8(x) (__PT_REGS_CAST(x)->__PT_PARM8_REG)
#define GO_PARAM9(x) (__PT_REGS_CAST(x)->regs[8])
#define GOROUTINE(x) (__PT_REGS_CAST(x)->regs[28])

#endif

static __always_inline void *get_argument(struct pt_regs *ctx, int index)
{
    switch (index)
    {
    case 1:
        return (void *)GO_PARAM1(ctx);
    case 2:
        return (void *)GO_PARAM2(ctx);
    case 3:
        return (void *)GO_PARAM3(ctx);
    case 4:
        return (void *)GO_PARAM4(ctx);
    case 5:
        return (void *)GO_PARAM5(ctx);
    case 6:
        return (void *)GO_PARAM6(ctx);
    case 7:
        return (void *)GO_PARAM7(ctx);
    case 8:
        return (void *)GO_PARAM8(ctx);
    case 9:
        return (void *)GO_PARAM9(ctx);
    default:
        return NULL;
    }
}

#endif
