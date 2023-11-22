#!/usr/bin/env bats

load ../../test_helpers/utilities

SCOPE="go.opentelemetry.io/auto/net/http"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: emits a span name '{http.method}' (per semconv)" {
  result=$(server_span_names_for ${SCOPE})
  assert_equal "$result" '"GET"'
}

@test "server :: includes http.method attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "server :: includes http.target attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.target\").value.stringValue")
  assert_equal "$result" '"/hello"'
}

@test "server :: includes http.status_code attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.status_code\").value.intValue")
  assert_equal "$result" '"200"'
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

@test "server :: net peer port is valid" {
  net_peer_port=$(server_spans_from_scope_named ${SCOPE} | jq '.attributes[] | select (.key == "net.peer.port") | .value.intValue')
  assert_regex "$net_peer_port" ${MATCH_A_PORT_NUMBER}
}

@test "client, server :: spans have same trace ID" {
  client_trace_id=$(server_spans_from_scope_named ${SCOPE} | jq ".traceId")
  server_trace_id=$(client_spans_from_scope_named ${SCOPE} | jq ".traceId")
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
