// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef GO_NET_H
#define GO_NET_H

#include "common.h"
#include "go_types.h"

typedef struct net_addr {
    u8 ip[16];
    u32 port;
} net_addr_t;

/*
type TCPAddr struct {
	IP   IP
	Port int
	Zone string // IPv6 scoped addressing zone
}
*/
const volatile u64 TCPAddr_IP_offset;
const volatile u64 TCPAddr_Port_offset;

static __always_inline long
get_tcp_net_addr_from_tcp_addr(struct pt_regs *ctx, net_addr_t *addr, void *tcpAddr_ptr) {
    go_slice_t ip;
    long res = bpf_probe_read_user(&ip, sizeof(ip), (void *)(tcpAddr_ptr + TCPAddr_IP_offset));
    if (res != 0) {
        bpf_printk("failed to read ip slice %d", res);
        return res;
    }

    u8 ip_slice_len = 4;
    if (ip.len != 4 && ip.len != 16) {
        bpf_printk("invalid ip slice length: %d", ip.len);
        return -1;
    }

    if (ip.len == 16) {
        ip_slice_len = 16;
    }

    res = bpf_probe_read_user(addr->ip, ip_slice_len, ip.array);
    if (res != 0) {
        bpf_printk("failed to read ip array");
        return res;
    }

    res = bpf_probe_read_user(
        &addr->port, sizeof(addr->port), (void *)(tcpAddr_ptr + TCPAddr_Port_offset));
    if (res != 0) {
        bpf_printk("failed to read port");
    }
    return res;
}

#endif
