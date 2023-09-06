#!/usr/bin/env bats

load ../../test_helpers/utilities

LIBRARY_NAME="database/sql"

@test "${LIBRARY_NAME} :: includes db.statement attribute" {
  result=$(span_attributes_for ${LIBRARY_NAME} | jq "select(.key == \"db.statement\").value.stringValue")
  assert_equal "$result" '"SELECT * FROM contacts"'
}

@test "${LIBRARY_NAME} :: trace ID present and valid in all spans" {
  trace_id=$(spans_from_scope_named ${LIBRARY_NAME} | jq ".traceId")
  assert_regex "$trace_id" ${MATCH_A_TRACE_ID}
}

@test "${LIBRARY_NAME} :: span ID present and valid in all spans" {
  span_id=$(spans_from_scope_named ${LIBRARY_NAME} | jq ".spanId")
  assert_regex "$span_id" ${MATCH_A_SPAN_ID}
}

@test "${LIBRARY_NAME} :: parent span ID present and valid in all spans" {
  parent_span_id=$(spans_from_scope_named ${LIBRARY_NAME} | jq ".parentSpanId")
  assert_regex "$parent_span_id" ${MATCH_A_SPAN_ID}
}

@test "${LIBRARY_NAME} :: expected (redacted) trace output" {
  redact_json
  assert_equal "$(git --no-pager diff ${BATS_TEST_DIRNAME}/traces.json)" ""
}
