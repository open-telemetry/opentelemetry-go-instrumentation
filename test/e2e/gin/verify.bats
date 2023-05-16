#!/usr/bin/env bats

load ../../test_helpers/utilities

LIBRARY_NAME="github.com/gin-gonic/gin"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "${LIBRARY_NAME} :: emits a span name '{http.method} {http.target}' (per semconv)" {
  result=$(span_names_for ${LIBRARY_NAME})
  assert_equal "$result" '"GET /hello-gin"'
}

@test "${LIBRARY_NAME} :: includes http.method attribute" {
  result=$(span_attributes_for ${LIBRARY_NAME} | jq "select(.key == \"http.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "${LIBRARY_NAME} :: includes http.target attribute" {
  result=$(span_attributes_for ${LIBRARY_NAME} | jq "select(.key == \"http.target\").value.stringValue")
  assert_equal "$result" '"/hello-gin"'
}

@test "${LIBRARY_NAME} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${LIBRARY_NAME} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${LIBRARY_NAME} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${LIBRARY_NAME} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${LIBRARY_NAME} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
