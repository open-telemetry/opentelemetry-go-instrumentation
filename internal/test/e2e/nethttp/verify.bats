#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/net/http"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: emits a span name '{http.request.method} {http.route}' (per semconv)" {
  result=$(server_span_names_for ${SCOPE})
  assert_equal "$result" '"GET /hello/{id}"'
}

@test "client :: emits a span name '{http.request.method}' (per semconv)" {
  result=$(client_span_names_for ${SCOPE})
  assert_equal "$result" '"GET"'
}

@test "server :: includes http.request.method attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.request.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "server :: includes url.path attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"url.path\").value.stringValue")
  assert_equal "$result" '"/hello/42"'
}

@test "client :: includes url.path attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"url.path\").value.stringValue")
  assert_equal "$result" '"/hello/42"'
}

@test "server :: includes http.response.status_code attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.response.status_code\").value.intValue")
  assert_equal "$result" '"200"'
}

@test "client :: includes http.response.status_code attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"http.response.status_code\").value.intValue")
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

@test "server :: network peer port is valid" {
  net_peer_port=$(server_spans_from_scope_named ${SCOPE} | jq '.attributes[] | select (.key == "network.peer.port") | .value.intValue')
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

@test "server :: includes server.address attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"server.address\").value.stringValue")
  assert_equal "$result" '"localhost"'
}

@test "server :: includes network.protocol.version attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"network.protocol.version\").value.stringValue")
  assert_equal "$result" '"1.1"'
}

@test "server :: includes network.peer.address attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"network.peer.address\").value.stringValue")
  assert_equal "$result" '"::1"'
}

@test "server :: includes http.route attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.route\").value.stringValue")
  assert_equal "$result" '"/hello/{id}"'
}

@test "client :: includes server.address attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"server.address\").value.stringValue")
  assert_equal "$result" '"localhost"'
}

@test "client :: includes server.port attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"server.port\").value.intValue")
  assert_equal "$result" '"8080"'
}

@test "client :: includes network.protocol.version attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"network.protocol.version\").value.stringValue")
  assert_equal "$result" '"1.1"'
}

@test "client :: includes url.full attribute" {
  result=$(client_span_attributes_for ${SCOPE} | jq "select(.key == \"url.full\").value.stringValue")
  assert_equal "$result" '"http://user@localhost:8080/hello/42?query=true#fragment"'
}
