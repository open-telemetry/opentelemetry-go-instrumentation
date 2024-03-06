#!/usr/bin/env bats

load ../../test_helpers/utilities

SCOPE="go.opentelemetry.io/auto/github.com/gin-gonic/gin"

@test "go-auto :: includes service.name in resource attributes" {
  result=$(resource_attributes_received | jq "select(.key == \"service.name\").value.stringValue")
  assert_equal "$result" '"sample-app"'
}

@test "${SCOPE} :: emits a span name '{http.request.method}' (per semconv)" {
  result=$(span_names_for ${SCOPE})
  assert_equal "$result" '"GET"'
}

@test "${SCOPE} :: includes http.request.method attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"http.request.method\").value.stringValue")
  assert_equal "$result" '"GET"'
}

@test "${SCOPE} :: includes url.path attribute" {
  result=$(span_attributes_for ${SCOPE} | jq "select(.key == \"url.path\").value.stringValue")
  assert_equal "$result" '"/hello-gin"'
}

@test "${SCOPE} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${SCOPE} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${SCOPE} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${SCOPE} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${SCOPE} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
