#!/usr/bin/env bats

load ../../test_helpers/utilities

SCOPE="go.opentelemetry.io/auto/google.golang.org/grpc"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: emits a span name 'SayHello'" {
  result=$(server_span_names_for ${SCOPE})
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "server :: includes rpc.system attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.system\").value.stringValue")
  assert_equal "$result" '"grpc"'
}

@test "server :: includes rpc.service attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.service\").value.stringValue")
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "server :: trace ID present and valid in all spans" {
  trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "server :: span ID present and valid in all spans" {
  span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "server :: parent span ID present and valid in all spans" {
  parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "client, server :: spans have same trace ID" {
  client_trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId")
  server_trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_equal "$server_trace_id" "$client_trace_id"
}

@test "client, server :: server span has client span as parent" {
  server_parent_span_id=$(server_spans_from_scope_named ${SCOPE} | jq ".parentSpanId")
  client_span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_equal "$client_span_id" "$server_parent_span_id"
}

@test "server :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}

@test "client :: emits a span name 'SayHello'" {
  result=$(client_span_names_for ${SCOPE})
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "client :: includes rpc.system attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.system\").value.stringValue")
  assert_equal "$result" '"grpc"'
}

@test "client :: includes rpc.service attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"rpc.service\").value.stringValue")
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "client :: trace ID present and valid in all spans" {
  trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "client :: span ID present and valid in all spans" {
  span_id=$(client_spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}
