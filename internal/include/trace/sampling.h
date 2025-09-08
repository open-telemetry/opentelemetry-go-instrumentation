// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _SAMPLING_H_
#define _SAMPLING_H_

#include "common.h"
#include "span_context.h"

#define MAX_SAMPLER_CONFIG_SIZE 256
#define MAX_SAMPLERS 32

typedef u32 sampler_id_t;

struct parent_based_config {
    sampler_id_t root;
    sampler_id_t remote_parent_sampled;
    sampler_id_t remote_parent_not_sampled;
    sampler_id_t local_parent_sampled;
    sampler_id_t local_parent_not_sampled;
};

enum sampler_type {
    // OpenTelemetry spec defined samplers
    ALWAYS_ON = 0,
    ALWAYS_OFF = 1,
    TRACE_ID_RATIO = 2,
    PARENT_BASED = 3,
    // Custom samplers

};

struct sampling_config {
    enum sampler_type type;
    union {
        u64 sampling_rate_numerator;
        struct parent_based_config parent_based;
        char buf[MAX_SAMPLER_CONFIG_SIZE];
    } config_data;
};

typedef struct sampling_parameters {
    struct span_context *psc;
    u8 *trace_id;
    // TODO: add more fields
} sampling_parameters_t;

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(key_size, sizeof(sampler_id_t));
    __uint(value_size, sizeof(struct sampling_config));
    __uint(max_entries, MAX_SAMPLERS);
} samplers_config_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(sampler_id_t));
    __uint(max_entries, 1);
} probe_active_sampler_map SEC(".maps");

static const u8 FLAG_SAMPLED = 1;

static __always_inline bool trace_flags_is_sampled(u8 flags) {
    return ((flags & FLAG_SAMPLED) == FLAG_SAMPLED);
}

static __always_inline bool is_sampled(struct span_context *ctx) {
    return trace_flags_is_sampled(ctx->TraceFlags);
}

// This value should be in sync with user-space code which configures the sampler
static const u64 sampling_rate_denominator = ((1ULL << 32) - 1);

static __always_inline bool _traceIDRatioSampler_should_sample(u64 sampling_rate_numerator,
                                                               u8 *trace_id) {
    if (sampling_rate_numerator == 0) {
        return false;
    }

    if (sampling_rate_numerator >= sampling_rate_denominator) {
        return true;
    }

    u64 trace_id_num = 0;
    __builtin_memcpy(&trace_id_num, &trace_id[8], 8);
    u64 trace_id_upper_bound = ((1ULL << 63) / sampling_rate_denominator) * sampling_rate_numerator;
    return (trace_id_num >> 1) < trace_id_upper_bound;
}

static __always_inline bool traceIDRatioSampler_should_sample(struct sampling_config *config,
                                                              sampling_parameters_t *params) {
    return _traceIDRatioSampler_should_sample(config->config_data.sampling_rate_numerator,
                                              params->trace_id);
}

static __always_inline bool alwaysOnSampler_should_sample(struct sampling_config *config,
                                                          sampling_parameters_t *params) {
    return true;
}

static __always_inline bool alwaysOffSampler_should_sample(struct sampling_config *config,
                                                           sampling_parameters_t *params) {
    return false;
}

static __always_inline bool parentBasedSampler_should_sample(struct sampling_config *config,
                                                             sampling_parameters_t *params) {
    sampler_id_t sampler_id;
    if (params->psc == NULL) {
        sampler_id = config->config_data.parent_based.root;
    } else {
        // TODO: once we add remote parent field to span context, we should check if it's remote or local
        // currently assuming local parent
        if (trace_flags_is_sampled(params->psc->TraceFlags)) {
            sampler_id = config->config_data.parent_based.local_parent_sampled;
        } else {
            sampler_id = config->config_data.parent_based.local_parent_not_sampled;
        }
    }

    struct sampling_config *base_config = bpf_map_lookup_elem(&samplers_config_map, &sampler_id);
    if (base_config == NULL) {
        bpf_printk("No sampler config found for parent based sampler\n");
        return false;
    }

    if (base_config->type == PARENT_BASED) {
        bpf_printk("Parent based sampler can't have parent based sampler as base\n");
        return false;
    }

    switch (base_config->type) {
    case ALWAYS_ON:
        return alwaysOnSampler_should_sample(base_config, params);
    case ALWAYS_OFF:
        return alwaysOffSampler_should_sample(base_config, params);
    case TRACE_ID_RATIO:
        return traceIDRatioSampler_should_sample(base_config, params);
    default:
        return false;
    }
}

static __always_inline bool should_sample(sampling_parameters_t *params) {
    u32 active_sampler_map_key = 0;
    sampler_id_t *active_sampler_id =
        bpf_map_lookup_elem(&probe_active_sampler_map, &active_sampler_map_key);
    if (active_sampler_id == NULL) {
        bpf_printk("No active sampler found\n");
        return false;
    }

    struct sampling_config *config = bpf_map_lookup_elem(&samplers_config_map, active_sampler_id);
    if (config == NULL) {
        bpf_printk("No sampler config found\n");
        return false;
    }

    switch (config->type) {
    case ALWAYS_ON:
        return alwaysOnSampler_should_sample(config, params);
    case ALWAYS_OFF:
        return alwaysOffSampler_should_sample(config, params);
    case TRACE_ID_RATIO:
        return traceIDRatioSampler_should_sample(config, params);
    case PARENT_BASED:
        return parentBasedSampler_should_sample(config, params);
    default:
        return false;
    }
}

#endif
