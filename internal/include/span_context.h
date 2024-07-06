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

#ifndef _SPAN_CONTEXT_H_
#define _SPAN_CONTEXT_H_

#include "utils.h"

#define SPAN_CONTEXT_STRING_SIZE 55
#define W3C_KEY_LENGTH 11 // length of the "traceparent" key
#define W3C_VAL_LENGTH 55
#define TRACE_ID_SIZE 16
#define TRACE_ID_STRING_SIZE 32
#define SPAN_ID_SIZE 8
#define SPAN_ID_STRING_SIZE 16
#define TRACE_FLAGS_SIZE 1
#define TRACE_FLAGS_STRING_SIZE 2

static const u8 FLAG_SAMPLED = 1;

struct span_context
{
    u8 TraceID[TRACE_ID_SIZE];
    u8 SpanID[SPAN_ID_SIZE];
    u8 TraceFlags;
    u8 padding[7];
};

// Fill the child span context based on the parent span context,
// generating a new span id and copying the trace id and trace flags
static __always_inline void get_span_context_from_parent(struct span_context *parent, struct span_context *child) {
    copy_byte_arrays(parent->TraceID, child->TraceID, TRACE_ID_SIZE);
    generate_random_bytes(child->SpanID, SPAN_ID_SIZE);
    child->TraceFlags = parent->TraceFlags;
}

// Fill the passed span context as root span context
static __always_inline void get_root_span_context(struct span_context *sc) {
    generate_random_bytes(sc->TraceID, TRACE_ID_SIZE);
    generate_random_bytes(sc->SpanID, SPAN_ID_SIZE);
    // currently we always sample
    sc->TraceFlags = FLAG_SAMPLED;
}

// TODO: remove this function once all the probes move to the above functions
static __always_inline struct span_context generate_span_context()
{
    struct span_context context = {};
    generate_random_bytes(context.TraceID, TRACE_ID_SIZE);
    generate_random_bytes(context.SpanID, SPAN_ID_SIZE);
    return context;
}

static __always_inline void span_context_to_w3c_string(struct span_context *ctx, char *buff)
{
    // W3C format: version (2 chars) - trace id (32 chars) - span id (16 chars) - sampled (2 chars)
    char *out = buff;

    // Write version
    *out++ = '0';
    *out++ = '0';
    *out++ = '-';

    // Write trace id
    bytes_to_hex_string(ctx->TraceID, TRACE_ID_SIZE, out);
    out += TRACE_ID_STRING_SIZE;
    *out++ = '-';

    // Write span id
    bytes_to_hex_string(ctx->SpanID, SPAN_ID_SIZE, out);
    out += SPAN_ID_STRING_SIZE;
    *out++ = '-';

    // Write trace flags
    bytes_to_hex_string(&ctx->TraceFlags, TRACE_FLAGS_SIZE, out);
}

static __always_inline void w3c_string_to_span_context(char *str, struct span_context *ctx)
{
    u32 trace_id_start_pos = 3;
    u32 span_id_start_pos = 36;
    u32 trace_flags_start_pos = 53;
    hex_string_to_bytes(str + trace_id_start_pos, TRACE_ID_STRING_SIZE, ctx->TraceID);
    hex_string_to_bytes(str + span_id_start_pos, SPAN_ID_STRING_SIZE, ctx->SpanID);
    hex_string_to_bytes(str + trace_flags_start_pos, TRACE_FLAGS_STRING_SIZE, &ctx->TraceFlags);
}

static __always_inline bool trace_flags_is_sampled(u8 flags)
{
    return ((flags & FLAG_SAMPLED) == FLAG_SAMPLED);
}

static __always_inline bool is_sampled(struct span_context *ctx)
{
    return trace_flags_is_sampled(ctx->TraceFlags);
}

#endif
