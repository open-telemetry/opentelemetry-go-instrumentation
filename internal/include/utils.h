// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef _EBPF_UTILS_H
#define _EBPF_UTILS_H

#include "bpf_helpers.h"


static __always_inline bool bpf_memcmp(char *s1, char *s2, s32 size)
{
    for (int i = 0; i < size; i++)
    {
        if (s1[i] != s2[i])
        {
            return false;
        }
    }

    return true;
}

// assumes s2 is all lowercase
static __always_inline int bpf_memicmp(const char *s1, const char *s2, s32 size) {
    for (int i = 0; i < size; i++) {
        if (s1[i] != s2[i] && s1[i] != (s2[i] - 32)) // compare with each uppercase character
        {
            return i + 1;
        }
    }

    return 0;
}

static __always_inline void generate_random_bytes(unsigned char *buff, u32 size)
{
    for (int i = 0; i < (size / 4); i++)
    {
        u32 random = bpf_get_prandom_u32();
        buff[(4 * i)] = (random >> 24) & 0xFF;
        buff[(4 * i) + 1] = (random >> 16) & 0xFF;
        buff[(4 * i) + 2] = (random >> 8) & 0xFF;
        buff[(4 * i) + 3] = random & 0xFF;
    }
}

char hex[16] = {'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'};
static __always_inline void bytes_to_hex_string(unsigned char *pin, u32 size, char *out)
{
    char *pout = out;
    int out_index = 0;
    for (u32 i = 0; i < size; i++)
    {
        *pout++ = hex[(*pin >> 4) & 0xF];
        *pout++ = hex[(*pin++) & 0xF];
    }
}

static __always_inline void hex_string_to_bytes(char *str, u32 size, unsigned char *out)
{
    for (int i = 0; i < (size / 2); i++)
    {
        char ch0 = str[2 * i];
        char ch1 = str[2 * i + 1];
        u8 nib0 = (ch0 & 0xF) + (ch0 >> 6) | ((ch0 >> 3) & 0x8);
        u8 nib1 = (ch1 & 0xF) + (ch1 >> 6) | ((ch1 >> 3) & 0x8);
        out[i] = (nib0 << 4) | nib1;
    }
}

static __always_inline void copy_byte_arrays(unsigned char *src, unsigned char *dst, u32 size)
{
    for (int i = 0; i < size; i++)
    {
        dst[i] = src[i];
    }
}

static __always_inline void bpf_memset(unsigned char *dst, u32 size, unsigned char value)
{
    for (int i = 0; i < size; i++)
    {
        dst[i] = value;
    }
}

static __always_inline bool bpf_is_zero(unsigned char *buff, u32 size)
{
    for (int i = 0; i < size; i++)
    {
        if (buff[i] != 0)
        {
            return false;
        }
    }

    return true;
}

#endif
