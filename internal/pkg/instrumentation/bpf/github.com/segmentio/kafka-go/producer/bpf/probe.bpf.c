// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include "arguments.h"
#include "trace/span_context.h"
#include "go_context.h"
#include "go_types.h"
#include "uprobe.h"
#include "trace/span_output.h"
#include "trace/start_span.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_CONCURRENT 50
// https://github.com/segmentio/kafka-go/blob/main/writer.go#L118
// TODO: (this value is directly impact the map sizes as well as the verification complexity)
// limitation on map entry size: https://github.com/iovisor/bcc/issues/2519#issuecomment-534359316
// the default value is 100, but it can be changed by the user
// we must specify a limit for the verifier
#define MAX_BATCH_SIZE 10
// https://github.com/apache/kafka/blob/0.10.2/core/src/main/scala/kafka/common/Topic.scala#L30C3-L30C34
#define MAX_TOPIC_SIZE 256
// No constraint on the key size, but we must have a limit for the verifier
#define MAX_KEY_SIZE 256

struct message_attributes_t {
    struct span_context sc;
    char topic[MAX_TOPIC_SIZE];
    char key[MAX_KEY_SIZE];
};

struct kafka_request_t {
    // common attributes to all the produced messages
    u64 start_time;
    u64 end_time;
    struct span_context psc;
    // attributes per message
    struct message_attributes_t msgs[MAX_BATCH_SIZE];
    char global_topic[MAX_TOPIC_SIZE];
    u64 valid_messages;
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct kafka_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} kafka_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct kafka_request_t));
    __uint(max_entries, 2);
} kafka_request_storage_map SEC(".maps");

// https://github.com/segmentio/kafka-go/blob/main/protocol/record.go#L48
struct kafka_header_t {
    struct go_string key;
    struct go_slice value;
};

// Injected in init
volatile const u64 message_key_pos;
volatile const u64 message_topic_pos;
volatile const u64 message_headers_pos;
volatile const u64 message_time_pos;

volatile const u64 writer_topic_pos;

#ifndef NO_HEADER_PROPAGATION
static __always_inline int build_contxet_header(struct kafka_header_t *header,
                                                struct span_context *span_ctx) {
    if (header == NULL || span_ctx == NULL) {
        bpf_printk("build_contxt_header: Invalid arguments");
        return -1;
    }

    // Prepare the key string for the user
    char key[W3C_KEY_LENGTH] = "traceparent";
    void *ptr = write_target_data(key, W3C_KEY_LENGTH);
    if (ptr == NULL) {
        bpf_printk("build_contxt_header: Failed to write key to user");
        return -1;
    }

    // build the go string of the key
    header->key.str = ptr;
    header->key.len = W3C_KEY_LENGTH;

    // Prepare the value string for the user
    char val[W3C_VAL_LENGTH];
    span_context_to_w3c_string(span_ctx, val);
    ptr = write_target_data(val, sizeof(val));
    if (ptr == NULL) {
        bpf_printk("build_contxt_header: Failed to write value to user");
        return -1;
    }

    // build the go slice of the value
    header->value.array = ptr;
    header->value.len = W3C_VAL_LENGTH;
    header->value.cap = W3C_VAL_LENGTH;
    bpf_printk("build_contxt_header success");
    return 0;
}

static __always_inline int inject_kafka_header(void *message, struct kafka_header_t *header) {
    append_item_to_slice(header, sizeof(*header), (void *)(message + message_headers_pos));
    return 0;
}
#endif

static __always_inline long
collect_kafka_attributes(void *message, struct message_attributes_t *attrs, bool collect_topic) {
    if (collect_topic) {
        // Topic might be globally set for a writer, or per message
        get_go_string_from_user_ptr(
            (void *)(message + message_topic_pos), attrs->topic, sizeof(attrs->topic));
    }

    // Key is a byte slice, first read the slice
    struct go_slice key_slice = {0};
    bpf_probe_read(&key_slice, sizeof(key_slice), (void *)(message + message_key_pos));
    u64 size_to_read = key_slice.len > MAX_KEY_SIZE ? MAX_KEY_SIZE : key_slice.len;
    size_to_read &= 0xFF;
    // Then read the actual key
    return bpf_probe_read(attrs->key, size_to_read, key_slice.array);
}

// This instrumentation attaches uprobe to the following function:
// func (w *Writer) WriteMessages(ctx context.Context, msgs ...Message) error
SEC("uprobe/WriteMessages")
int uprobe_WriteMessages(struct pt_regs *ctx) {
    // In Go, "..." is equivalent to passing a slice: https://go.dev/ref/spec#Passing_arguments_to_..._parameters
    void *writer = get_argument(ctx, 1);
    void *msgs_array = get_argument(ctx, 4);
    u64 msgs_array_len = (u64)get_argument(ctx, 5);

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    void *key = (void *)GOROUTINE(ctx);

    void *kafka_request_ptr = bpf_map_lookup_elem(&kafka_events, &key);
    if (kafka_request_ptr != NULL) {
        bpf_printk("uprobe/WriteMessages already tracked with the current context");
        return 0;
    }

    u32 zero_id = 0;
    struct kafka_request_t *zero_kafka_request =
        bpf_map_lookup_elem(&kafka_request_storage_map, &zero_id);
    if (zero_kafka_request == NULL) {
        bpf_printk("uuprobe/WriteMessages: zero_kafka_request is NULL");
        return 0;
    }

    u32 actual_id = 1;
    // Zero the span we are about to build, eBPF doesn't support memset of large structs (more than 1024 bytes)
    bpf_map_update_elem(&kafka_request_storage_map, &actual_id, zero_kafka_request, BPF_ANY);
    // Get a pointer to the zeroed span
    struct kafka_request_t *kafka_request =
        bpf_map_lookup_elem(&kafka_request_storage_map, &actual_id);
    if (kafka_request == NULL) {
        bpf_printk("uprobe/WriteMessages: Failed to get kafka_request");
        return 0;
    }

    kafka_request->start_time = bpf_ktime_get_ns();

    start_span_params_t start_span_params = {
        .ctx = ctx,
        .go_context = &go_context,
        .psc = &kafka_request->psc,
        .sc = &kafka_request->msgs[0].sc,
        .get_parent_span_context_fn = NULL,
        .get_parent_span_context_arg = NULL,
    };
    start_span(&start_span_params);

    // Try to get a global topic from Writer
    bool global_topic = get_go_string_from_user_ptr((void *)(writer + writer_topic_pos),
                                                    kafka_request->global_topic,
                                                    sizeof(kafka_request->global_topic));

    void *msg_ptr = msgs_array;
    struct kafka_header_t header = {0};
    // This is hack to get the message size. This calculation is based on the following assumptions:
    // 1. "Time" is the last field in the message struct. This looks to be correct for all the versions according to
    //      https://github.com/segmentio/kafka-go/blob/v0.2.3/message.go#L24C2-L24C6
    // 2. the time.Time struct is 24 bytes. This looks to be correct for all the reasnobaly latest versions according to
    //      https://github.com/golang/go/blame/master/src/time/time.go#L135
    // In the future if more libraries will need to get structs sizes we probably want to have similar
    // mechanism to the one we have for the offsets
    u16 msg_size = message_time_pos + 8 + 8 + 8;
    kafka_request->valid_messages = 0;
    // Iterate over the messages
    for (u64 i = 0; i < MAX_BATCH_SIZE; i++) {
        if (i >= msgs_array_len) {
            break;
        }
        // Optionally collect the topic, and always collect key
        collect_kafka_attributes(msg_ptr, &kafka_request->msgs[i], !global_topic);
        // Generate span id for each message
        if (i > 0) {
            generate_random_bytes(kafka_request->msgs[i].sc.SpanID, SPAN_ID_SIZE);
            // Copy the trace id and trace flags from the first message. This means the sampling decision is done on the first message,
            // and all the messages in the batch will have the same trace id and trace flags.
            kafka_request->msgs[i].sc.TraceFlags = kafka_request->msgs[0].sc.TraceFlags;
            __builtin_memcpy(kafka_request->msgs[i].sc.TraceID,
                             kafka_request->msgs[0].sc.TraceID,
                             TRACE_ID_SIZE);
        }

#ifndef NO_HEADER_PROPAGATION
        // Build the header
        if (build_contxet_header(&header, &kafka_request->msgs[i].sc) != 0) {
            bpf_printk("uprobe/WriteMessages: Failed to build header");
            return 0;
        }
        // Inject the header
        inject_kafka_header(msg_ptr, &header);
#endif
        kafka_request->valid_messages++;
        msg_ptr = msg_ptr + msg_size;
    }

    bpf_map_update_elem(&kafka_events, &key, kafka_request, 0);
    // don't need to start tracking the span, as we don't have a context to propagate locally
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (w *Writer) WriteMessages(ctx context.Context, msgs ...Message) error
SEC("uprobe/WriteMessages")
int uprobe_WriteMessages_Returns(struct pt_regs *ctx) {
    u64 end_time = bpf_ktime_get_ns();
    void *key = (void *)GOROUTINE(ctx);

    struct kafka_request_t *kafka_request = bpf_map_lookup_elem(&kafka_events, &key);
    if (kafka_request == NULL) {
        bpf_printk("kafka_request is null\n");
        return 0;
    }
    kafka_request->end_time = end_time;

    output_span_event(ctx, kafka_request, sizeof(*kafka_request), &kafka_request->msgs[0].sc);
    bpf_map_delete_elem(&kafka_events, &key);
    // don't need to stop tracking the span, as we don't have a context to propagate locally
    return 0;
}
