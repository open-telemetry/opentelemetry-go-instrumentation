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
// https://github.com/apache/kafka/blob/0.10.2/core/src/main/scala/kafka/common/Topic.scala#L30C3-L30C34
#define MAX_TOPIC_SIZE 256
// No constraint on the key size, but we must have a limit for the verifier
#define MAX_KEY_SIZE 256
#define MAX_CONSUMER_GROUP_SIZE 128

struct kafka_request_t {
    BASE_SPAN_PROPERTIES
    char topic[MAX_TOPIC_SIZE];
    char key[MAX_KEY_SIZE];
    char consumer_group[MAX_CONSUMER_GROUP_SIZE];
    s64 offset;
    s64 partition;
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct kafka_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} kafka_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, void *);
    __uint(max_entries, MAX_CONCURRENT);
} goroutine_to_go_context SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, void *);
    __uint(max_entries, MAX_CONCURRENT);
} kafka_reader_to_conn SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(struct kafka_request_t));
    __uint(max_entries, 1);
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
volatile const u64 message_partition_pos;
volatile const u64 message_offset_pos;

volatile const u64 reader_config_pos;
volatile const u64 reader_config_group_id_pos;

#define MAX_HEADERS 20

static __always_inline long
extract_span_context_from_headers(void *message, struct span_context *parent_span_context) {
    // Read the headers slice descriptor
    void *headers = (void *)(message + message_headers_pos);
    struct go_slice headers_slice = {0};
    bpf_probe_read(&headers_slice, sizeof(headers_slice), headers);

    char key[W3C_KEY_LENGTH] = "traceparent";
    char current_key[W3C_KEY_LENGTH];

    for (u64 i = 0; i < headers_slice.len; i++) {
        if (i >= MAX_HEADERS) {
            break;
        }
        // Read the header
        struct kafka_header_t header = {0};
        bpf_probe_read(&header, sizeof(header), headers_slice.array + (i * sizeof(header)));
        // Check if it is the traceparent header
        if (header.key.len == W3C_KEY_LENGTH && header.value.len == W3C_VAL_LENGTH) {
            bpf_probe_read_user(current_key, sizeof(current_key), header.key.str);
            if (bpf_memcmp(key, current_key, sizeof(key))) {
                // Found the traceparent header, extract the span context
                char val[W3C_VAL_LENGTH];
                bpf_probe_read(val, W3C_VAL_LENGTH, header.value.array);
                w3c_string_to_span_context(val, parent_span_context);
                return 0;
            }
        }
    }

    return -1;
}

// This instrumentation attaches uprobe to the following function:
// func (r *Reader) FetchMessage(ctx context.Context) (Message, error)
SEC("uprobe/FetchMessage")
int uprobe_FetchMessage(struct pt_regs *ctx) {
    /* FetchMessage is a blocking function, hence its execution time is not a good indication for the time it took to handle the message.
    Instead, we use the entry to this function to end the span which was started when it's last call returned. (A typical consumer calls FetchMessage in a loop)
    A less confusing way of looking at it is as follows 
    1. Entry to FetchMessage
    2. internal kafka code before blocking
    3. Blocking wait for message
    4. internal kafka code after blocking
    5. Return from FetchMessage
    Steps 2-4 are executed in a separate goroutine from the one the user of the library.
    */
    void *reader = get_argument(ctx, 1);
    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);
    void *goroutine = (void *)GOROUTINE(ctx);
    struct kafka_request_t *kafka_request = bpf_map_lookup_elem(&kafka_events, &goroutine);
    if (kafka_request == NULL) {
        // The current goroutine has no kafka request,
        // this can happen in the first time FetchMessage is called
        // Save the context for the return probe for in-process context propagation
        goto save_context;
    }

    get_go_string_from_user_ptr((void *)(reader + reader_config_pos + reader_config_group_id_pos),
                                kafka_request->consumer_group,
                                sizeof(kafka_request->consumer_group));
    kafka_request->end_time = bpf_ktime_get_ns();

    output_span_event(ctx, kafka_request, sizeof(*kafka_request), &kafka_request->sc);
    stop_tracking_span(&kafka_request->sc, &kafka_request->psc);
    bpf_map_delete_elem(&kafka_events, &goroutine);

save_context:
    // Save the context for the return probe
    bpf_map_update_elem(&goroutine_to_go_context, &goroutine, &go_context.data, 0);
    return 0;
}

// This instrumentation attaches uprobe to the following function:
// func (r *Reader) FetchMessage(ctx context.Context) (Message, error)
SEC("uprobe/FetchMessage")
int uprobe_FetchMessage_Returns(struct pt_regs *ctx) {
    /* The FetchMessage function returns a message to the user after it read it from a channel.
    The user consuming this message will handle it after this probe,
    thus it is a good place to start track the span corresponds to this message. In addition we save the message
    in a hash map to be read by the entry probe of FetchMessage, which will end this span */
    void *goroutine = (void *)GOROUTINE(ctx);
    u32 map_id = 0;
    struct kafka_request_t *kafka_request =
        bpf_map_lookup_elem(&kafka_request_storage_map, &map_id);
    if (kafka_request == NULL) {
        bpf_printk("uuprobe/sendMessage: kafka_request is NULL");
        return 0;
    }
    kafka_request->start_time = bpf_ktime_get_ns();
    // The message returned on the stack since it returned as a struct and not a pointer
    void *message = (void *)(PT_REGS_SP(ctx) + 8);

    struct go_iface go_context = {0};
    get_Go_context(ctx, 2, 0, true, &go_context);

    // Get the parent span context from the message headers
    start_span_params_t start_span_params = {
        .ctx = ctx,
        .sc = &kafka_request->sc,
        .psc = &kafka_request->psc,
        .go_context = &go_context,
        .get_parent_span_context_fn = extract_span_context_from_headers,
        .get_parent_span_context_arg = message,
    };
    start_span(&start_span_params);

    // Collecting message attributes
    // topic
    get_go_string_from_user_ptr(
        (void *)(message + message_topic_pos), kafka_request->topic, sizeof(kafka_request->topic));
    // partition
    bpf_probe_read(&kafka_request->partition,
                   sizeof(kafka_request->partition),
                   (void *)(message + message_partition_pos));
    // offset
    bpf_probe_read(&kafka_request->offset,
                   sizeof(kafka_request->offset),
                   (void *)(message + message_offset_pos));
    // Key is a byte slice, first read the slice descriptor
    struct go_slice key_slice = {0};
    bpf_probe_read(&key_slice, sizeof(key_slice), (void *)(message + message_key_pos));
    u64 size_to_read = key_slice.len > MAX_KEY_SIZE ? MAX_KEY_SIZE : key_slice.len;
    size_to_read &= 0xFF;
    // Then read the actual key
    bpf_probe_read(kafka_request->key, size_to_read, key_slice.array);

    bpf_map_update_elem(&kafka_events, &goroutine, kafka_request, 0);

    // We are start tracking the consumer span in the return probe,
    // hence we can't read Go's context directly from the registers as we usually do.
    // Using the goroutine address as a key to the map that contains the context.
    void *context_data_ptr = bpf_map_lookup_elem(&goroutine_to_go_context, &goroutine);
    if (context_data_ptr != NULL) {
        bpf_probe_read_kernel(&context_data_ptr, sizeof(context_data_ptr), context_data_ptr);
        start_tracking_span(context_data_ptr, &kafka_request->sc);
        bpf_map_delete_elem(&goroutine_to_go_context, &goroutine);
    }

    return 0;
}
