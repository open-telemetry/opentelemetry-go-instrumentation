#!/usr/bin/env bats

load ../../test_helpers/utilities

SERVER_LIBRARY_NAME="google.golang.org/grpc/server"
CLIENT_LIBRARY_NAME="google.golang.org/grpc"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "${SERVER_LIBRARY_NAME} :: emits a span name 'SayHello'" {
  result=$(span_names_for ${SERVER_LIBRARY_NAME})
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "${SERVER_LIBRARY_NAME} :: includes rpc.system attribute" {
  result=$(span_attributes_for ${SERVER_LIBRARY_NAME} | jq "select(.key == \"rpc.system\").value.stringValue")
  assert_equal "$result" '"grpc"'
}

@test "${SERVER_LIBRARY_NAME} :: includes rpc.service attribute" {
  result=$(span_attributes_for ${SERVER_LIBRARY_NAME} | jq "select(.key == \"rpc.service\").value.stringValue")
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "${SERVER_LIBRARY_NAME} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${SERVER_LIBRARY_NAME} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${SERVER_LIBRARY_NAME} :: parent span ID present and valid in all spans" {
  parent_span_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "${CLIENT_LIBRARY_NAME}, ${SERVER_LIBRARY_NAME} :: spans have same trace ID" {
  client_trace_id=$(spans_from_scope_named ${CLIENT_LIBRARY_NAME} | jq ".traceId")
  server_trace_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".traceId")
  assert_equal "$server_trace_id" "$client_trace_id"
}

@test "${CLIENT_LIBRARY_NAME}, ${SERVER_LIBRARY_NAME} :: server span has client span as parent" {
  server_parent_span_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".parentSpanId")
  client_span_id=$(spans_from_scope_named ${CLIENT_LIBRARY_NAME} | jq ".spanId")
  assert_equal "$client_span_id" "$server_parent_span_id"
}

@test "${SERVER_LIBRARY_NAME} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}

@test "${CLIENT_LIBRARY_NAME} :: emits a span name 'SayHello'" {
  result=$(span_names_for ${SERVER_LIBRARY_NAME})
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "${CLIENT_LIBRARY_NAME} :: includes rpc.system attribute" {
  result=$(span_attributes_for ${SERVER_LIBRARY_NAME} | jq "select(.key == \"rpc.system\").value.stringValue")
  assert_equal "$result" '"grpc"'
}

@test "${CLIENT_LIBRARY_NAME} :: includes rpc.service attribute" {
  result=$(span_attributes_for ${SERVER_LIBRARY_NAME} | jq "select(.key == \"rpc.service\").value.stringValue")
  assert_equal "$result" '"/helloworld.Greeter/SayHello"'
}

@test "${CLIENT_LIBRARY_NAME} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${CLIENT_LIBRARY_NAME} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${SERVER_LIBRARY_NAME} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}
