// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "go_context.h"
#include "go_types.h"
#include "trace/span_context.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_CONCURRENT 50
// TODO: tune this. This is a just a guess, but we should be able to determine
// the maximum size of a span (based on limits) and set this. Ideally, we could
// also look into a tiered allocation strategy so we do not over allocate
// space (i.e. small, medium, large data sizes).
#define MAX_SIZE 1024

// Injected const.
volatile const u64 span_context_trace_id_pos;
volatile const u64 span_context_span_id_pos;
volatile const u64 span_context_trace_flags_pos;

struct otel_span_t {
    struct span_context sc;
    struct span_context psc;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct otel_span_t));
    __uint(max_entries, 2);
} otel_span_storage_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, void*);
	__type(value, struct otel_span_t);
	__uint(max_entries, MAX_CONCURRENT);
} active_spans_by_span_ptr SEC(".maps");

struct event_t {
	u32 size;
	char data[MAX_SIZE];
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct event_t));
    __uint(max_entries, 1);
} new_event SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// zero_span will zero out the passed otel_span.
static __always_inline long zero_span(struct otel_span_t *otel_span) {
	// eBPF does not support using memset structs greater than 1024 bytes in
	// size. Instead, use the otel_span_storage_map. The otel_span_storage_map
	// is designed to hold two entries:
	//   - a zero span
	//   - an active span
	//
	// What we are doing here is:
	//
	//   1. Get a copy of the zero span.
	//   2. Set our copy of the zero span to the active span.
	//   3. Set *otel_span to the active span.
	//
	// This approach is not concurrent safe and requires synchronization on the
	// thread this is called from.

    u32 zero_key = 0;
    struct otel_span_t *zero = bpf_map_lookup_elem(&otel_span_storage_map, &zero_key);
    if (zero == NULL) {
		// Invalid otel_span_storage_map setup.
		return -1;
    }

    u32 active_key = 1;
    bpf_map_update_elem(&otel_span_storage_map, &active_key, zero, 0);

    struct otel_span_t *active_span = bpf_map_lookup_elem(&otel_span_storage_map, &active_key);
    if (active_span == NULL) {
		return -2;
    }

	*otel_span = *active_span;
	return 0;
}

static __always_inline long write_span_context(void *go_sc, struct span_context *sc) {
	bpf_printk("write_span_context: go_sc: %p", go_sc);
	if (go_sc == NULL) {
		bpf_printk("write_span_context: NULL go_sc");
		return -1;
	}

	long rc = 0;
	void *tid = (void *)(go_sc + span_context_trace_id_pos);
	long ret = bpf_probe_write_user(tid, &sc->TraceID, TRACE_ID_SIZE);
	if (ret != 0) {
		rc--;
		bpf_printk("write_span_context: failed to write trace ID: %ld", ret);
	}

	void *sid = (void *)(go_sc + span_context_span_id_pos);
	ret = bpf_probe_write_user(sid, &sc->SpanID, SPAN_ID_SIZE);
	if (ret != 0) {
		rc--;
		bpf_printk("write_span_context: failed to write span ID: %ld", ret);
	}

	void *flags = (void *)(go_sc + span_context_trace_flags_pos);
	ret = bpf_probe_write_user(flags, &sc->TraceFlags, TRACE_FLAGS_SIZE);
	if (ret != 0) {
		rc--;
		bpf_printk("write_span_context: failed to write trace flags: %ld", ret);
	}

	return rc;
}

static __always_inline long span_context_from_Go(void *go_span_context, struct span_context *sc) {
	if (go_span_context == NULL) {
		bpf_printk("span_context_from_Go: empty span context");
		struct span_context zero = {0};
		*sc = zero;
		return 0;
	}

	long rc = 0;
	void *tid = (void *)(go_span_context + span_context_trace_id_pos);
	long ret = bpf_probe_read(&sc->TraceID, TRACE_ID_SIZE, tid);
	if (ret != 0) {
		rc--;
		bpf_printk("span_context_from_Go: failed to read trace ID", ret);
	}

	void *sid = (void *)(go_span_context + span_context_span_id_pos);
	ret = bpf_probe_read(&sc->SpanID, SPAN_ID_SIZE, sid);
	if (ret != 0) {
		rc--;
		bpf_printk("span_context_from_Go: failed to read span ID", ret);
	}

	void *flags = (void *)(go_span_context + span_context_trace_flags_pos);
	ret = bpf_probe_read(&sc->TraceFlags, TRACE_FLAGS_SIZE, flags);
	if (ret != 0) {
		rc--;
		bpf_printk("span_context_from_Go: failed to read trace flags", ret);
	}

	return rc;
}

SEC("uprobe/Tracer_start")
int uprobe_Tracer_start(struct pt_regs *ctx) {
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);

	struct otel_span_t otel_span = {0};
	long rc = zero_span(&otel_span) < 0;
	if (rc != 0) {
		return -1;
	}

    start_span_params_t params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &otel_span.psc,
        .sc = &otel_span.sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL, // Default to new root. 
    };

    start_span(&params);

	void *parent_span_context = get_argument(ctx, 5);
	rc = write_span_context(parent_span_context, &otel_span.psc);
	if (rc != 0) {
		bpf_printk("failed to write parent span context: %ld", rc);
	}

	if (!is_sampled(params.sc)) {
		// Default SDK behaviour is to sample everything. Only set the sampled
		// value to false when needed.
		void *sampled_ptr_val = get_argument(ctx, 6);
		if (sampled_ptr_val == NULL) {
			bpf_printk("nil sampled pointer");
		} else {
			bool false_val = false;
			rc = bpf_probe_write_user(sampled_ptr_val, &false_val, sizeof(bool));
			if (rc != 0) {
				bpf_printk("bpf_probe_write_user: failed to write sampled value: %ld", rc);
			} else {
				bpf_printk("wrote sampled value");
			}
		}
	}

	void *span_context_ptr_val = get_argument(ctx, 7);
	rc = write_span_context(span_context_ptr_val, &otel_span.sc);
	if (rc != 0) {
		bpf_printk("failed to write span context: %ld", rc);
	}

	void *span_ptr_val = get_argument(ctx, 4);
    bpf_map_update_elem(&active_spans_by_span_ptr, &span_ptr_val, &otel_span, 0);
    start_tracking_span(go_context.data, &otel_span.sc);

	return 0;
}

SEC("uprobe/Span_ended")
int uprobe_Span_ended(struct pt_regs *ctx) {
    void *span_ptr = get_argument(ctx, 1);
    struct otel_span_t *span = bpf_map_lookup_elem(&active_spans_by_span_ptr, &span_ptr);
    if (span == NULL) {
        return 0;
    }
    stop_tracking_span(&span->sc, &span->psc);
    bpf_map_delete_elem(&active_spans_by_span_ptr, &span_ptr);

	u64 len = (u64)get_argument(ctx, 3);
	if (len > MAX_SIZE) {
		bpf_printk("span data too large: %d", len);
		return -1;
	}
	if (len == 0) {
		bpf_printk("empty span data");
		return 0;
	}

	void *data_ptr = get_argument(ctx, 2);
	if (data_ptr == NULL) {
		bpf_printk("empty span data");
		return 0;
	}

	u32 key = 0;
	struct event_t *event = bpf_map_lookup_elem(&new_event, &key);
	if (event == NULL) {
		bpf_printk("failed to initialize new event");
		return -2;
	}
	event->size = (u32)len;

	if (event->size < MAX_SIZE) {
		long rc = bpf_probe_read(&event->data, event->size, data_ptr);
		if (rc < 0) {
			bpf_printk("failed to read encoded span data");
			return -3;
		}
	} else {
		bpf_printk("read too large: %d", event->size);
		return -4;
	}

	bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, event, sizeof(*event));

	return 0;
}
