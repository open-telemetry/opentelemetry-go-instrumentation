#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
 
# Copied from https://github.com/cilium/ebpf/blob/v0.15.0/examples/headers/update.sh

# MIT License
# 
# Copyright (c) 2017 Nathan Sweet
# Copyright (c) 2018, 2019 Cloudflare
# Copyright (c) 2019 Authors of Cilium
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
# 
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
# 
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

# Version of libbpf to fetch headers from
LIBBPF_VERSION="1.4.5"

# The headers we want
prefix="libbpf-$LIBBPF_VERSION"
headers=(
    "${prefix}/LICENSE"
    "${prefix}/LICENSE.BSD-2-Clause"
    "${prefix}/LICENSE.LGPL-2.1"
    "${prefix}/src/bpf_helpers.h"
    "${prefix}/src/bpf_helper_defs.h"
    "${prefix}/src/bpf_tracing.h"
)

# Fetch libbpf release and extract the desired headers
curl -sL "https://github.com/libbpf/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz" | \
    tar -xz --xform='s#.*/##' "${headers[@]}"
