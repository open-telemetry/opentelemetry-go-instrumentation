#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/github.com/segmentio/kafka-go"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_each "$result" '"sample-app"'
}

@test "kafka producer,consumer :: valid {messaging.system} for all spans" {
  messaging_systems=$(span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.system\").value.stringValue")
  result_separated=$(echo $messaging_systems | sed 's/\n/,/g')
  assert_equal "$result_separated" '"kafka" "kafka" "kafka"'
}

@test "producer :: valid {messaging.destination.name} for all spans" {
  topics=$(producer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.destination.name\").value.stringValue" | sort )
  result_separated=$(echo $topics | sed 's/\n/,/g')
  assert_equal "$result_separated" '"topic1" "topic2"'
}

@test "consumer :: valid {messaging.destination.name} for all spans" {
  topics=$(consumer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.destination.name\").value.stringValue" | sort)
  assert_equal "$topics" '"topic1"'
}

@test "producer :: valid {messaging.kafka.message.key} for all spans" {
  keys=$(producer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.kafka.message.key\").value.stringValue" | sort )
  result_separated=$(echo $keys | sed 's/\n/,/g')
  assert_equal "$result_separated" '"key1" "key2"'
}

@test "producer :: valid {messaging.batch.message_count} for all spans" {
  batch_sizes=$(producer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.batch.message_count\").value.intValue")
  result_separated=$(echo $batch_sizes | sed 's/\n/,/g')
  assert_equal "$result_separated" '"2" "2"'
}

@test "consumer :: valid {messaging.kafka.message.key}" {
  topics=$(consumer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.kafka.message.key\").value.stringValue" | sort )
  assert_equal "$topics" '"key1"'
}

@test "consumer :: valid {messaging.destination.partition.id}" {
  partition=$(consumer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.destination.partition.id\").value.stringValue" | sort )
  assert_equal "$partition" '"0"'
}

@test "consumer :: valid {messaging.kafka.offset}" {
  offset=$(consumer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.kafka.offset\").value.intValue" | sort )
  assert_equal "$offset" '"0"'
}

@test "consumer :: valid {messaging.kafka.consumer.group}" {
  consumer_group=$(consumer_span_attributes_for ${SCOPE} | jq "select(.key == \"messaging.consumer.group.name\").value.stringValue" | sort )
  assert_equal "$consumer_group" '"some group id"'
}

@test "producer :: valid span names" {
  span_names=$(producer_spans_from_scope_named ${SCOPE} | jq ".name" | sort)
  result_separated=$(echo $span_names | sed 's/\n/,/g')
  assert_equal "$result_separated" '"topic1 publish" "topic2 publish"'
}

@test "consumer :: valid span names" {
  span_names=$(consumer_spans_from_scope_named ${SCOPE} | jq ".name")
  result_separated=$(echo $span_names | sed 's/\n/,/g')
  assert_equal "$result_separated" '"topic1 receive"'
}

@test "consumer :: trace ID present and valid in all spans" {
  trace_id=$(consumer_spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "consumer :: span ID present and valid in all spans" {
  span_id=$(consumer_spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "consumer :: parent span ID present and valid in all spans" {
  parent_span_id=$(consumer_spans_from_scope_named ${SCOPE} | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "producer, consumer :: spans have same trace ID" {
  producer_trace_id=$(producer_spans_from_scope_named ${SCOPE} | jq ".traceId" | uniq)
  consumer_trace_id=$(consumer_spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_equal "$producer_trace_id" "$consumer_trace_id"
}

@test "producer, consumer :: consumer span has producer span as parent" {
  consumer_parent_span_id=$(consumer_spans_from_scope_named ${SCOPE} | jq ".parentSpanId")
  producer_span_id=$(producer_spans_from_scope_named ${SCOPE} | jq "select(.name == \"topic1 publish\")" | jq ."spanId")
  assert_equal "$producer_span_id" "$consumer_parent_span_id"
}
