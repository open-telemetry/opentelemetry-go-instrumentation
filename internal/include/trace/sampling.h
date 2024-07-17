// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _SAMPLING_H_
#define _SAMPLING_H_

#include "common.h"
#include "span_context.h"

typedef struct sampling_parameters {
    struct span_context *psc;
    u8 *trace_id;
    // TODO: add more fields
} sampling_parameters_t;

static __always_inline bool should_sample(sampling_parameters_t *params) {
    // TODO
    return true;
}

#endif
