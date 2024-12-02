#!/usr/bin/env bats

load ../../test_helpers/utilities.sh

SCOPE="go.opentelemetry.io/auto/github.com/go-chi/chi/v5"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "server :: includes http.request.method attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.request.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "server :: includes url.path attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"url.path\").value.stringValue")
  assert_equal "$result" '"/hello/42"'
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

@test "server :: includes http.route attribute" {
  result=$(server_span_attributes_for ${SCOPE} | jq "select(.key == \"http.route\").value.stringValue")
  assert_equal "$result" '"/hello/{id}"'
}

@test "server :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
