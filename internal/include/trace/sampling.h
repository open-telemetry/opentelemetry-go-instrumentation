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