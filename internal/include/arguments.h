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

// Every span created by the auto instrumentation should contain end timestamp.
// This end timestamp is recorded at the end of probed function by editing the struct that was created at the beginning.
// Usually probes create an eBPF map to store the span struct and retrieve it at the end of the function.
// Consistent key is used as a key for that map.
static __always_inline void *get_consistent_key(struct pt_regs *ctx)
{
    return (void *)GOROUTINE(ctx);
}

#endif
