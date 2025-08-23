// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _START_SPAN_H_
#define _START_SPAN_H_

#include "common.h"
#include "span_context.h"
#include "sampling.h"

// function for getting the parent span context, the result is stored in the passed span context.
// the function should return 0 if the parent span context is found, negative value otherwise.
// Each probe can potentially have a different way of getting the parent span context,
// this is useful for incoming requests (http, kafka, etc.) where the parent span context needs to be extracted from the
// incoming request.
// The handle param can be used to pass any data needed to get the parent span context.
typedef long (*get_parent_sc_fn)(void *handle, struct span_context *psc);

typedef struct start_span_params {
    struct pt_regs *ctx;
    struct go_iface *go_context;
    struct span_context *psc;
    struct span_context *sc;
    // function for getting the parent span context, the result is stored in the passed span context.
    get_parent_sc_fn get_parent_span_context_fn;
    // argument to be passed to the get_parent_span_context_fn
    void *get_parent_span_context_arg;
} start_span_params_t;

// Start a new span, setting the parent span context if found.
// Generate a new span context for the new span. Perform sampling decision and set the TraceFlags accordingly.
static __always_inline void start_span(start_span_params_t *params) {
    long found_parent = -1;
    if (params->get_parent_span_context_fn != NULL) {
        found_parent =
            params->get_parent_span_context_fn(params->get_parent_span_context_arg, params->psc);
    } else {
        struct span_context *local_psc = get_parent_span_context(params->go_context);
        if (local_psc != NULL) {
            found_parent = 0;
            *(params->psc) = *local_psc;
        }
    }

    u8 parent_trace_flags = 0;
    if (found_parent == 0) {
        get_span_context_from_parent(params->psc, params->sc);
        parent_trace_flags = params->psc->TraceFlags;
    } else {
        get_root_span_context(params->sc);
    }

    sampling_parameters_t sampling_params = {
        .trace_id = params->sc->TraceID,
        .psc = (found_parent == 0) ? params->psc : NULL,
    };
    bool sample = should_sample(&sampling_params);
    if (sample) {
        params->sc->TraceFlags = (parent_trace_flags) | (FLAG_SAMPLED);
    } else {
        params->sc->TraceFlags = (parent_trace_flags) & (~FLAG_SAMPLED);
    }
}

#endif
